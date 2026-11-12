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
  // Fake route: /_/cart/items/{id} → /api/cart/items/{id}
  final routes = [
    BffRoute({'POST', 'PATCH'}, RegExp(r'^/_/cart/items(/\d+)?$'), (m) => '/api/cart/items${m[1] ?? ''}'),
  ];

  late List<http.Request> seen;
  late http.Response apiResponse;

  Future<Response> send(
    String method,
    String path, {
    Map<String, String> headers = const {},
    String body = '',
  }) async {
    seen = [];
    final client = MockClient((req) async {
      seen.add(req);
      return apiResponse;
    });
    final handler = bff(cfg, client, routes)((_) => Response.ok('page'));
    return handler(
      Request(
        method,
        Uri.parse('http://shop.example.com$path'),
        // Caddy https e `X-Forwarded-Proto` dey (Origin er scheme er sathe mele).
        headers: {'host': 'shop.example.com', 'x-forwarded-proto': 'https', ...headers},
        body: body,
      ),
    );
  }

  const same = {'origin': 'https://shop.example.com'};

  setUp(() => apiResponse = http.Response('{"id":1}', 201, headers: {'content-type': 'application/json'}));

  group('BFF (10.9)', () {
    for (final (name, headers) in [
      ('Origin nai', <String, String>{}),
      ('onno Origin', {'origin': 'https://evil.example'}),
      ('Origin e onno port', {'origin': 'https://shop.example.com:8443'}),
      ('Sec-Fetch-Site cross-site', {'sec-fetch-site': 'cross-site'}),
      ('Origin null', {'origin': 'null'}),
      ('Origin e http (site https)', {'origin': 'http://shop.example.com'}),
      ('Origin null ar cross-site', {'origin': 'null', 'sec-fetch-site': 'cross-site'}),
    ]) {
      test('$name → 403, API te jay na', () async {
        final res = await send('POST', '/_/cart/items', headers: headers);
        expect(res.statusCode, 403);
        expect(res.headers['cache-control'], 'no-store');
        expect(seen, isEmpty);
      });
    }

    test('Origin chhara Sec-Fetch-Site same-origin hole chole', () async {
      final res = await send('POST', '/_/cart/items', headers: {'sec-fetch-site': 'same-origin'}, body: '{}');
      expect(res.statusCode, 201);
    });

    test('no-referrer page er form: Origin null + Sec-Fetch-Site same-origin chole', () async {
      final res = await send(
        'POST',
        '/_/cart/items',
        headers: {'origin': 'null', 'sec-fetch-site': 'same-origin'},
        body: '{}',
      );
      expect(res.statusCode, 201);
    });

    test('Referer er path // diye (protocol-relative) hole redirect / e, onno site e na', () async {
      final res = await send(
        'POST',
        '/_/cart/items',
        headers: {
          ...same,
          'content-type': 'application/x-www-form-urlencoded',
          'referer': 'https://shop.example.com//evil.com/x',
        },
        body: 'variant_id=5',
      );
      expect(res.headers['location'], '/');
    });

    test('boro body (Go er MaxBody 1 MB er beshi) 413, bhul form encoding 400; API te na', () async {
      final big = await send('POST', '/_/cart/items', headers: same, body: 'x' * ((1 << 20) + 1));
      expect(big.statusCode, 413);
      final bad = await send(
        'POST',
        '/_/cart/items',
        headers: {...same, 'content-type': 'application/x-www-form-urlencoded'},
        body: 'a=%zz',
      );
      expect(bad.statusCode, 400);
      expect(jsonDecode(await bad.readAsString())['error']['code'], 'validation_failed');
      expect(seen, isEmpty);
    });

    test('header forward: shop, IP, cookie → token, Referer theke bhasha; browser er cookie ar bhasha na', () async {
      await send(
        'POST',
        '/_/cart/items',
        headers: {
          ...same,
          'x-real-ip': '203.0.113.7',
          'cookie': '__Host-sid=tok1; __Host-cart=cart9; lang=bn',
          'referer': 'https://shop.example.com/en/p/lal-chal',
          'accept-language': 'bn-BD',
          'content-type': 'application/json',
          'idempotency-key': 'k-1',
        },
        body: '{"variant_id":5,"quantity":2}',
      );
      final r = seen.single;
      expect(r.method, 'POST');
      expect(r.url.toString(), 'http://127.0.0.1:8090/api/cart/items');
      expect(r.headers['X-Shop-Host'], 'shop.example.com');
      expect(r.headers['X-Client-IP'], '203.0.113.7');
      expect(r.headers['X-Internal-Auth'], 's3cret');
      expect(r.headers['Authorization'], 'Bearer tok1');
      expect(r.headers['X-Cart-Key'], 'cart9');
      expect(r.headers['Accept-Language'], 'en');
      expect(r.headers['Idempotency-Key'], 'k-1');
      expect(r.headers.keys.map((k) => k.toLowerCase()), isNot(contains('cookie')));
      expect(r.body, '{"variant_id":5,"quantity":2}');
    });

    test('cookie ar Referer na thakle Authorization/X-Cart-Key nai, bhasha bn', () async {
      await send('PATCH', '/_/cart/items/7', headers: same, body: '{}');
      final r = seen.single;
      expect(r.url.path, '/api/cart/items/7');
      expect(r.headers.containsKey('Authorization'), isFalse);
      expect(r.headers.containsKey('X-Cart-Key'), isFalse);
      expect(r.headers['Accept-Language'], 'bn');
    });

    test('Go API er status ar error JSON hubohu, no-store', () async {
      const err = '{"error":{"code":"out_of_stock","message":"x","details":{"available":1}}}';
      apiResponse = http.Response(err, 409, headers: {'content-type': 'application/json'});
      final res = await send('POST', '/_/cart/items', headers: same, body: '{}');
      expect(res.statusCode, 409);
      expect(await res.readAsString(), err);
      expect(res.headers['content-type'], 'application/json');
      expect(res.headers['cache-control'], 'no-store');
    });

    test('ochena BFF path ba method → 404 no-store, API te na', () async {
      for (final (m, p) in [('POST', '/_/nai'), ('DELETE', '/_/cart/items/1')]) {
        final res = await send(m, p, headers: same);
        expect(res.statusCode, 404);
        expect(res.headers['cache-control'], 'no-store');
        expect(jsonDecode(await res.readAsString())['error']['code'], 'not_found');
      }
      expect(seen, isEmpty);
    });

    test('/_/ er baire page handler e jay', () async {
      final res = await send('POST', '/p/lal-chal', headers: same);
      expect(await res.readAsString(), 'page');
    });

    test('JS chhara form: JSON e forward, 2xx e 303 Referer e', () async {
      final res = await send(
        'POST',
        '/_/cart/items',
        headers: {
          ...same,
          'content-type': 'application/x-www-form-urlencoded',
          'referer': 'https://shop.example.com/p/lal-chal?x=1',
        },
        body: 'variant_id=5&note=%E0%A6%95',
      );
      expect(seen.single.headers['Content-Type'], startsWith('application/json'));
      expect(jsonDecode(seen.single.body), {'variant_id': '5', 'note': 'ক'});
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/p/lal-chal?x=1');
      expect(res.headers['cache-control'], 'no-store');
    });

    test('JS chhara form e error: 303 form er page e ?error=<code>; onno host er Referer na', () async {
      apiResponse = http.Response('{"error":{"code":"out_of_stock","message":""}}', 409);
      final res = await send(
        'POST',
        '/_/cart/items',
        headers: {...same, 'content-type': 'application/x-www-form-urlencoded', 'referer': 'https://evil.example/x'},
        body: 'variant_id=5',
      );
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/?error=out_of_stock');
    });

    test('API te jete na parle 502 network_error', () async {
      final client = MockClient((_) => throw http.ClientException('down'));
      final res = await bff(cfg, client, routes)((_) => Response.ok(''))(
        Request(
          'POST',
          Uri.parse('http://shop.example.com/_/cart/items'),
          headers: {'host': 'shop.example.com', 'x-forwarded-proto': 'https', ...same},
        ),
      );
      expect(res.statusCode, 502);
      expect(jsonDecode(await res.readAsString())['error']['code'], 'network_error');
    });
  });
}
