import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shelf/shelf.dart';
import 'package:test/test.dart';
import 'package:website/services/bff.dart';
import 'package:website/services/config.dart';

void main() {
  final cfg = Config.fromEnv({
    'API_BASE_URL': 'http://127.0.0.1:8090',
    'JASPR_INTERNAL_SECRET': 's3cret',
    'IMAGE_BASE_URL': 'https://img.example.com',
    'IMAGE_RESIZE': 'none',
  });
  const sid = '__Host-sid=TOK2ABC; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=2592000';
  const cleared = '__Host-sid=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0';

  late List<http.Request> seen;
  late http.Response apiResponse;

  Future<Response> send(
    String method,
    String path, {
    Map<String, String> headers = const {'origin': 'https://shop.example.com'},
    String body = '',
  }) async {
    seen = [];
    final client = MockClient((req) async {
      seen.add(req);
      return apiResponse;
    });
    return bff(cfg, client, bffRoutes)((_) => Response.ok('page'))(
      Request(
        method,
        Uri.parse('http://shop.example.com$path'),
        headers: {'host': 'shop.example.com', 'x-forwarded-proto': 'https', ...headers},
        body: body,
      ),
    );
  }

  group('BFF auth (10.3, 10.9, 10.11)', () {
    test('otp/send: body hubohu Go te, uttor hubohu, no-store, cookie na', () async {
      apiResponse = http.Response('{"sent":true}', 200, headers: {'content-type': 'application/json'});
      const body = '{"phone":"01712345678","purpose":"login","turnstile_token":"t"}';
      final res = await send('POST', '/_/auth/otp/send', body: body);
      final r = seen.single;
      expect(r.method, 'POST');
      expect(r.url.toString(), 'http://127.0.0.1:8090/api/auth/otp/send');
      expect(r.body, body);
      expect(r.headers['X-Shop-Host'], 'shop.example.com');
      expect(res.statusCode, 200);
      expect(await res.readAsString(), '{"sent":true}');
      expect(res.headers['cache-control'], 'no-store');
      expect(res.headers.containsKey('set-cookie'), isFalse);
    });

    test('otp/send 429: status, error JSON ar Retry-After hubohu', () async {
      const err = '{"error":{"code":"rate_limited","message":""}}';
      apiResponse = http.Response(
        err,
        429,
        headers: {'content-type': 'application/json', 'retry-after': '120'},
      );
      final res = await send('POST', '/_/auth/otp/send', body: '{}');
      expect(res.statusCode, 429);
      expect(await res.readAsString(), err);
      expect(res.headers['retry-after'], '120');
    });

    test('otp/verify: token __Host-sid cookie te (10.3 flag), body theke muche, private, no-store', () async {
      apiResponse = http.Response(
        '{"token":"TOK2ABC","user":{"phone":"01712345678","name":null,"locale":"bn"}}',
        200,
        headers: {'content-type': 'application/json'},
      );
      final res = await send('POST', '/_/auth/otp/verify', body: '{"phone":"01712345678","code":"123456"}');
      expect(seen.single.url.path, '/api/auth/otp/verify');
      expect(res.statusCode, 200);
      expect(res.headers['set-cookie'], sid);
      final out = await res.readAsString();
      expect(out, isNot(contains('TOK2ABC')));
      expect(jsonDecode(out), {
        'user': {'phone': '01712345678', 'name': null, 'locale': 'bn'},
      });
      expect(res.headers['cache-control'], 'private, no-store');
    });

    test('otp/verify error: Go er status/JSON hubohu, cookie na', () async {
      const err = '{"error":{"code":"otp_invalid","message":""}}';
      apiResponse = http.Response(err, 400, headers: {'content-type': 'application/json'});
      final res = await send('POST', '/_/auth/otp/verify', body: '{}');
      expect(res.statusCode, 400);
      expect(await res.readAsString(), err);
      expect(res.headers.containsKey('set-cookie'), isFalse);
      expect(res.headers['cache-control'], 'no-store');
    });

    test('otp/verify e Go er uttor e thik token nai (ba cookie bhanga okkhor) → 502, cookie na', () async {
      for (final body in ['{"user":{}}', '{"token":"a;b","user":{}}', '{"token":1}', 'nope']) {
        apiResponse = http.Response(body, 200);
        final res = await send('POST', '/_/auth/otp/verify', body: '{}');
        expect(res.statusCode, 502, reason: body);
        expect(res.headers.containsKey('set-cookie'), isFalse, reason: body);
        expect(await res.readAsString(), isNot(contains('token')), reason: body);
      }
    });

    test('logout: cookie theke Bearer Go te, cookie Max-Age=0, Go er 204', () async {
      apiResponse = http.Response('', 204);
      final res = await send(
        'POST',
        '/_/auth/logout',
        headers: {'origin': 'https://shop.example.com', 'cookie': '__Host-sid=TOK2ABC'},
      );
      expect(seen.single.url.path, '/api/auth/logout');
      expect(seen.single.headers['Authorization'], 'Bearer TOK2ABC');
      expect(res.statusCode, 204);
      expect(res.headers['set-cookie'], cleared);
      expect(res.headers['cache-control'], 'private, no-store');
    });

    test('logout e Go 401 (session expire) holeo cookie muche, error hubohu', () async {
      const err = '{"error":{"code":"session_expired","message":""}}';
      apiResponse = http.Response(err, 401, headers: {'content-type': 'application/json'});
      final res = await send('POST', '/_/auth/logout');
      expect(res.statusCode, 401);
      expect(await res.readAsString(), err);
      expect(res.headers['set-cookie'], cleared);
    });

    test('PATCH /_/account → PATCH /api/me, Bearer shoho, uttor hubohu', () async {
      apiResponse = http.Response('{"phone":"01712345678","name":"Rahim"}', 200);
      final res = await send(
        'PATCH',
        '/_/account',
        headers: {'origin': 'https://shop.example.com', 'cookie': '__Host-sid=TOK2ABC'},
        body: '{"name":"Rahim"}',
      );
      final r = seen.single;
      expect(r.method, 'PATCH');
      expect(r.url.path, '/api/me');
      expect(r.headers['Authorization'], 'Bearer TOK2ABC');
      expect(r.body, '{"name":"Rahim"}');
      expect(res.statusCode, 200);
      expect(await res.readAsString(), '{"phone":"01712345678","name":"Rahim"}');
    });

    test('JS chhara logout form: 303 Referer e, cookie muche', () async {
      apiResponse = http.Response('', 204);
      final res = await send(
        'POST',
        '/_/auth/logout',
        headers: {
          'origin': 'https://shop.example.com',
          'cookie': '__Host-sid=TOK2ABC',
          'content-type': 'application/x-www-form-urlencoded',
          'referer': 'https://shop.example.com/account',
        },
      );
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/account');
      expect(res.headers['set-cookie'], cleared);
    });

    for (final path in ['/_/auth/otp/send', '/_/auth/otp/verify', '/_/auth/logout']) {
      test('$path onno Origin e 403, Go te jay na, cookie na', () async {
        apiResponse = http.Response('{"token":"TOK2ABC","user":{}}', 200);
        final res = await send('POST', path, headers: {'origin': 'https://evil.example'});
        expect(res.statusCode, 403);
        expect(seen, isEmpty);
        expect(res.headers.containsKey('set-cookie'), isFalse);
      });
    }

    test('GET ba onno method e auth route nai (404)', () async {
      for (final (m, p) in [('GET', '/_/auth/otp/verify'), ('GET', '/_/account'), ('DELETE', '/_/auth/logout')]) {
        final res = await send(m, p);
        expect(res.statusCode, 404, reason: '$m $p');
      }
      expect(seen, isEmpty);
    });
  });
}
