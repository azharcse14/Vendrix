import 'package:http/http.dart' as http;

/// Protiti `401` e [onUnauthorized] (screen er session expire → login, 11.3); response hubohu.
class SessionClient extends http.BaseClient {
  final http.Client inner;
  final void Function() onUnauthorized;
  SessionClient(this.inner, this.onUnauthorized);

  @override
  Future<http.StreamedResponse> send(http.BaseRequest request) async {
    final res = await inner.send(request);
    if (res.statusCode == 401) onUnauthorized();
    return res;
  }

  @override
  void close() => inner.close();
}
