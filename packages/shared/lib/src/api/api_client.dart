import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;

/// API er shob bhul ekta type e (7.1, v1.12). Logic [code] diye; [message] shudhu dekhano
/// (client er `network_error`, `timeout` er lekha `Translator` e `error.<code>`).
class ApiError implements Exception {
  /// HTTP status (network ba timeout e 0).
  final int status;

  /// 7.1 er stable code.
  final String code;

  /// Server er lekha (thakle).
  final String message;

  /// Field → karon.
  final Map<String, String> fields;

  /// Baki tottho.
  final Map<String, dynamic> details;

  /// `429` er `Retry-After`.
  final Duration? retryAfter;

  /// ApiError.
  const ApiError(
    this.status,
    this.code, {
    this.message = '',
    this.fields = const {},
    this.details = const {},
    this.retryAfter,
  });

  @override
  String toString() => 'ApiError($status, $code)';
}

/// Go API er client (7.1). HTTP `Client` inject hoy (admin e `BrowserClient`, test e `MockClient`).
/// Auto retry na (POST idempotent na; dorkar hole app nije).
class ApiClient {
  /// API er base URL.
  final Uri baseUrl;

  /// HTTP client.
  final http.Client client;

  /// `Accept-Language` (`bn` ba `en`).
  final String locale;

  /// Protiti request er header (jemon `X-Shop-Host`, Jaspr er `X-Client-IP`).
  final Map<String, String> headers;

  /// Ek request er shima (default 15 second).
  final Duration timeout;

  /// ApiClient.
  const ApiClient(
    this.baseUrl,
    this.client, {
    this.locale = 'bn',
    this.headers = const {},
    this.timeout = const Duration(seconds: 15),
  });

  /// GET; JSON ferot.
  Future<Object?> get(String path, {Map<String, String>? query}) =>
      send('GET', path, query: query);

  /// Je kono method; `body` JSON hoye jay; `204` e null. Bhul hole [ApiError].
  Future<Object?> send(
    String method,
    String path, {
    Object? body,
    Map<String, String>? query,
  }) async {
    final req =
        http.Request(
            method,
            baseUrl.resolve(path).replace(queryParameters: query),
          )
          ..headers.addAll({
            ...headers,
            'Accept-Language': locale,
            'Accept': 'application/json',
          });
    if (body != null) {
      req.headers['Content-Type'] = 'application/json; charset=utf-8';
      req.body = jsonEncode(body);
    }
    final http.Response res;
    try {
      res = await client
          .send(req)
          .then(http.Response.fromStream)
          .timeout(timeout);
    } on TimeoutException {
      throw const ApiError(0, 'timeout');
    } on http.ClientException {
      throw const ApiError(0, 'network_error');
    }
    if (res.statusCode == 204) return null;
    Object? data;
    try {
      data = jsonDecode(utf8.decode(res.bodyBytes));
    } on FormatException {
      throw ApiError(res.statusCode, 'internal'); // jemon proxy er HTML 502
    }
    if (res.statusCode >= 200 && res.statusCode < 300) return data;
    final e =
        data is Map<String, dynamic> && data['error'] is Map<String, dynamic>
        ? data['error'] as Map<String, dynamic>
        : const <String, dynamic>{};
    final retry = int.tryParse(res.headers['retry-after'] ?? '');
    throw ApiError(
      res.statusCode,
      e['code'] as String? ?? 'internal',
      message: e['message'] as String? ?? '',
      fields: (e['fields'] as Map<String, dynamic>? ?? const {})
          .cast<String, String>(),
      details: e['details'] as Map<String, dynamic>? ?? const {},
      retryAfter: retry == null ? null : Duration(seconds: retry),
    );
  }
}
