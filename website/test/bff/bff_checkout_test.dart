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
    'TURNSTILE_SITE_KEY': '1x00000000000000000000AA',
  });
  const key = 'AAAABBBBCCCCDDDDEEEEFFFF22';
  const token = 'QWERTYUIOPASDFGHJKLZXCVBNM';
  const idem = 'abcdefghijklmnopqrstuvwxyz';

  late List<http.Request> seen;
  late http.Response apiResponse;

  Future<Response> send(
    String method,
    String path, {
    String? cookie,
    String origin = 'https://shop.example.com',
    Map<String, String> headers = const {},
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
        headers: {
          'host': 'shop.example.com',
          'x-forwarded-proto': 'https',
          'x-real-ip': '203.0.113.9',
          'origin': origin,
          'cookie': ?cookie,
          ...headers,
        },
        body: body,
      ),
    );
  }

  setUp(() => apiResponse = http.Response('{"ok":true}', 200, headers: {'content-type': 'application/json'}));

  group('BFF checkout ar order (10.9, 10.13)', () {
    for (final (path, api) in [
      ('/_/checkout/quote', '/api/checkout/quote'),
      ('/_/checkout', '/api/orders'),
      ('/_/orders/$token/cancel', '/api/orders/$token/cancel'),
      ('/_/confirm/$token', '/api/confirm/$token'),
      ('/_/track', '/api/orders/track'),
    ]) {
      test('POST $path → $api: body hubohu, no-store', () async {
        const body = '{"district":"Dhaka","area":"Mirpur"}';
        final res = await send('POST', path, body: body);
        final r = seen.single;
        expect(r.method, 'POST');
        expect(r.url.toString(), 'http://127.0.0.1:8090$api');
        expect(r.body, body);
        expect(r.headers['X-Shop-Host'], 'shop.example.com');
        expect(r.headers['X-Internal-Auth'], 's3cret');
        expect(r.headers['X-Client-IP'], '203.0.113.9');
        expect(res.statusCode, 200);
        expect(await res.readAsString(), '{"ok":true}');
        expect(res.headers['cache-control'], 'no-store');
      });
    }

    test('guest: __Host-cart → X-Cart-Key, Bearer na', () async {
      await send('POST', '/_/checkout/quote', cookie: '__Host-cart=$key', body: '{}');
      expect(seen.single.headers['X-Cart-Key'], key);
      expect(seen.single.headers.containsKey('Authorization'), isFalse);
    });

    test('login: __Host-sid → Bearer, X-Cart-Key na', () async {
      final res = await send('POST', '/_/checkout', cookie: '__Host-sid=TOK2ABC; __Host-cart=$key', body: '{}');
      expect(seen.single.headers['Authorization'], 'Bearer TOK2ABC');
      expect(seen.single.headers.containsKey('X-Cart-Key'), isFalse);
      expect(res.headers['cache-control'], 'no-store');
    });

    test('Idempotency-Key header Go te forward', () async {
      await send('POST', '/_/checkout', headers: {'idempotency-key': idem}, body: '{}');
      expect(seen.single.headers['Idempotency-Key'], idem);
    });

    for (final (status, err) in [
      (409, '{"error":{"code":"otp_required","message":"","details":{"phone_masked":"+88017****5678"}}}'),
      (409, '{"error":{"code":"price_changed","message":"","details":{"total":42000,"subtotal":40000}}}'),
      (422, '{"error":{"code":"idempotency_key_reused","message":""}}'),
      (404, '{"error":{"code":"not_found","message":""}}'),
    ]) {
      test('Go er $status hubohu ferot: $err', () async {
        apiResponse = http.Response(err, status, headers: {'content-type': 'application/json'});
        final res = await send('POST', '/_/checkout', body: '{}');
        expect(res.statusCode, status);
        expect(await res.readAsString(), err);
        expect(res.headers['content-type'], 'application/json');
        expect(res.headers['cache-control'], 'no-store');
      });
    }

    test('429 e Retry-After ferot', () async {
      apiResponse = http.Response(
        '{"error":{"code":"rate_limited","message":""}}',
        429,
        headers: {'retry-after': '60'},
      );
      final res = await send('POST', '/_/track', body: '{}');
      expect(res.statusCode, 429);
      expect(res.headers['retry-after'], '60');
    });

    test('onno Origin e 403, Go te jay na', () async {
      for (final path in ['/_/checkout', '/_/checkout/quote', '/_/orders/$token/cancel', '/_/confirm/$token']) {
        final res = await send('POST', path, origin: 'https://evil.example', body: '{}');
        expect(res.statusCode, 403, reason: path);
        expect(seen, isEmpty, reason: path);
      }
    });

    test('token e bhul okkhor (path traversal) ba GET e route nai (404)', () async {
      for (final (m, p) in [
        ('POST', '/_/orders/a.b/cancel'),
        ('POST', '/_/orders/%2E%2E/cancel'),
        ('POST', '/_/confirm/'),
        ('GET', '/_/checkout'),
        ('GET', '/_/confirm/$token'),
        // Page SSR e Go theke ney (10.9 e GET route nai).
        ('GET', '/_/me/orders'),
        ('GET', '/_/delivery-zones'),
      ]) {
        final res = await send(m, p);
        expect(res.statusCode, 404, reason: '$m $p');
        expect(seen, isEmpty, reason: '$m $p');
      }
    });
  });
}
