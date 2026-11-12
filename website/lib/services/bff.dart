import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:shelf/shelf.dart';

import 'api.dart';
import 'config.dart';
import 'locale.dart';

/// BFF er ekta route (10.9): [methods] ar [path] mille Go API er [api] path e forward.
class BffRoute {
  /// HTTP method.
  final Set<String> methods;

  /// `/_/...` path (puro mil).
  final RegExp path;

  /// Mil theke Go API er path.
  final String Function(Match) api;

  /// BffRoute.
  const BffRoute(this.methods, this.path, this.api);
}

// Go API er `httpx.MaxBody` (12.2): er beshi body memory te pora hoy na.
const _maxBody = 1 << 20;

/// Asol route: M3 e skeleton, prothom route M4 e (`/_/auth/otp/*`, 10.10).
const bffRoutes = <BffRoute>[];

/// Request er cookie (naam → value).
Map<String, String> parseCookies(String? header) => {
  for (final c in (header ?? '').split(';'))
    if (c.indexOf('=') case final i when i > 0) c.substring(0, i).trim(): c.substring(i + 1).trim(),
};

/// BFF (10.9): `/_/...` Go API te forward; baki [inner] e. Dam ba stock hishab na, shudhu forward (Rule 1).
Middleware bff(Config cfg, http.Client client, List<BffRoute> routes) =>
    (inner) => (request) async {
      final path = '/${request.url.path}';
      if (!path.startsWith('/_/')) return inner(request);
      final host = request.headers['host'] ?? '';
      if (request.method != 'GET' && request.method != 'HEAD' && !_sameOrigin(request.headers, host)) {
        return _error(403, 'forbidden');
      }
      final (route, match) = _find(routes, request.method, path);
      if (route == null) return _error(404, 'not_found');

      final cookies = parseCookies(request.headers['cookie']);
      final referer = _sameOriginPath(request.headers['referer'], host);
      final form = (request.headers['content-type'] ?? '').startsWith('application/x-www-form-urlencoded');
      final bytes = <int>[];
      await for (final chunk in request.read()) {
        bytes.addAll(chunk);
        if (bytes.length > _maxBody) return _error(413, 'validation_failed');
      }
      final String raw;
      final Map<String, String>? fields;
      try {
        raw = utf8.decode(bytes);
        fields = form ? Uri.splitQueryString(raw) : null;
      } on FormatException {
        return _error(400, 'validation_failed');
      } on ArgumentError {
        return _error(400, 'validation_failed');
      }
      final api = http.Request(request.method, cfg.apiBaseUrl.resolve(route.api(match!)))
        ..headers.addAll({
          ...apiHeaders(request.headers, cfg),
          'Accept': 'application/json',
          'Accept-Language': localeOf(Uri.parse(referer).path).locale,
          if (cookies['__Host-sid'] case final sid?) 'Authorization': 'Bearer $sid',
          'X-Cart-Key': ?cookies['__Host-cart'],
          'Idempotency-Key': ?request.headers['idempotency-key'],
          if (form || raw.isNotEmpty) 'Content-Type': 'application/json; charset=utf-8',
        })
        ..body = fields != null ? jsonEncode(fields) : raw;

      final http.Response res;
      try {
        res = await client.send(api).then(http.Response.fromStream).timeout(const Duration(seconds: 15));
      } on TimeoutException {
        return _error(504, 'timeout');
      } on http.ClientException {
        return _error(502, 'network_error');
      }
      if (form) {
        // POST-redirect-GET; error hole form er page e `?error=<code>` (10.9 niyom 4)
        final back = Uri.parse(referer);
        final ok = res.statusCode >= 200 && res.statusCode < 300;
        final to = ok ? back : back.replace(queryParameters: {...back.queryParameters, 'error': _code(res.body)});
        return Response(303, headers: {'location': to.toString(), 'cache-control': 'no-store'});
      }
      return Response(
        res.statusCode,
        body: res.bodyBytes,
        headers: {'content-type': ?res.headers['content-type'], 'cache-control': 'no-store'},
      );
    };

// CSRF (10.9 niyom 1): Origin thakle seta hubohu ei site er origin (scheme shoho, [originOf]); na thakle ba
// `null` (no-referrer page er form, 10.1) hole Sec-Fetch-Site same-origin.
bool _sameOrigin(Map<String, String> headers, String host) {
  final origin = headers['origin'];
  if (origin != null && origin != 'null') return host.isNotEmpty && origin == originOf(headers);
  return headers['sec-fetch-site'] == 'same-origin';
}

// Nijer page er path+query (redirect ar bhasha); onno host ba nai hole `/` (open redirect na).
String _sameOriginPath(String? referer, String host) {
  final u = Uri.tryParse(referer ?? '');
  // `//evil.com` ba `\` path browser e onno host (protocol-relative).
  if (u == null || u.authority != host || host.isEmpty || u.path.startsWith('//') || u.path.contains(r'\')) {
    return '/';
  }
  return u.hasQuery ? '${u.path}?${u.query}' : (u.path.isEmpty ? '/' : u.path);
}

(BffRoute?, Match?) _find(List<BffRoute> routes, String method, String path) {
  for (final r in routes) {
    if (!r.methods.contains(method)) continue;
    if (r.path.firstMatch(path) case final m?) return (r, m);
  }
  return (null, null);
}

String _code(String body) {
  try {
    final code = (jsonDecode(body) as Map)['error']['code'];
    if (code is String) return code;
  } catch (_) {}
  return 'internal';
}

Response _error(int status, String code) => Response(
  status,
  body: jsonEncode({
    'error': {'code': code, 'message': ''},
  }),
  headers: {'content-type': 'application/json', 'cache-control': 'no-store'},
);
