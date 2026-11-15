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
    'TURNSTILE_SITE_KEY': '1x00000000000000000000AA',
  });
  // Go er `rand.Text()` er format (`^[A-Z2-7]{26}$`, 8.4).
  const key = 'AAAABBBBCCCCDDDDEEEEFFFF22';
  const newKey = 'ZZZZYYYYXXXXWWWWVVVVUUUU77';
  String cartCookie(String k) => '__Host-cart=$k; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=2592000';
  const cartCleared = '__Host-cart=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0';
  const wlCleared = 'wl=; Path=/; Secure; SameSite=Lax; Max-Age=0';
  const cartJson = '{"items":[],"count":0,"subtotal":0,"notices":[]}';

  late List<http.Request> seen;
  // Go er uttor path onujayi; na thakle cart.
  late Map<String, http.Response> api;

  Future<Response> send(
    String method,
    String path, {
    String? cookie,
    String origin = 'https://shop.example.com',
    String? form,
    String? referer,
    String body = '',
  }) async {
    seen = [];
    final client = MockClient((req) async {
      seen.add(req);
      return api[req.url.path] ?? http.Response(cartJson, 200, headers: {'content-type': 'application/json'});
    });
    return bff(cfg, client, bffRoutes)((_) => Response.ok('page'))(
      Request(
        method,
        Uri.parse('http://shop.example.com$path'),
        headers: {
          'host': 'shop.example.com',
          'x-forwarded-proto': 'https',
          'origin': origin,
          'cookie': ?cookie,
          'referer': ?referer,
          if (form != null) 'content-type': 'application/x-www-form-urlencoded',
        },
        body: form ?? body,
      ),
    );
  }

  List<String> setCookies(Response res) => res.headersAll['set-cookie'] ?? const [];

  setUp(() => api = {});

  group('BFF cart (10.3, 10.9, 10.12)', () {
    test('notun cart_key: __Host-cart cookie (10.3 flag, 30 din), body theke muche, private, no-store', () async {
      api['/api/cart/items'] = http.Response(
        '{"items":[{"variant_id":5,"quantity":2}],"count":1,"subtotal":36000,"notices":[],"cart_key":"$newKey"}',
        200,
        headers: {'content-type': 'application/json'},
      );
      final res = await send('POST', '/_/cart/items', body: '{"variant_id":5,"quantity":2}');
      final r = seen.single;
      expect(r.method, 'POST');
      expect(r.url.toString(), 'http://127.0.0.1:8090/api/cart/items');
      expect(r.headers.containsKey('X-Cart-Key'), isFalse);
      expect(r.body, '{"variant_id":5,"quantity":2}');
      expect(res.statusCode, 200);
      expect(setCookies(res), [cartCookie(newKey)]);
      final out = await res.readAsString();
      expect(out, isNot(contains(newKey)));
      expect((jsonDecode(out) as Map).containsKey('cart_key'), isFalse);
      expect(jsonDecode(out)['subtotal'], 36000);
      expect(res.headers['cache-control'], 'private, no-store');
    });

    test('cookie er key X-Cart-Key e; protiti write e cookie abar (sliding 30 din)', () async {
      for (final (method, path, apiPath) in [
        ('POST', '/_/cart/items', '/api/cart/items'),
        ('PATCH', '/_/cart/items/7', '/api/cart/items/7'),
        ('DELETE', '/_/cart/items/7', '/api/cart/items/7'),
      ]) {
        final res = await send(method, path, cookie: 'lang=bn; __Host-cart=$key', body: '{"quantity":1}');
        final r = seen.single;
        expect(r.method, method, reason: path);
        expect(r.url.path, apiPath);
        expect(r.headers['X-Cart-Key'], key);
        expect(r.headers.containsKey('Authorization'), isFalse);
        expect(res.statusCode, 200);
        expect(await res.readAsString(), cartJson);
        expect(setCookies(res), [cartCookie(key)], reason: '$method $path');
        expect(res.headers['cache-control'], 'private, no-store');
      }
    });

    test('login thakle __Host-sid Bearer, X-Cart-Key na, cart cookie set na', () async {
      final res = await send('PATCH', '/_/cart/items/7', cookie: '__Host-sid=TOK2ABC; __Host-cart=$key', body: '{}');
      final r = seen.single;
      expect(r.headers['Authorization'], 'Bearer TOK2ABC');
      expect(r.headers.containsKey('X-Cart-Key'), isFalse);
      expect(setCookies(res), isEmpty);
    });

    test('bhul format er cookie key Go te jay na, abar set o hoy na', () async {
      for (final bad in ['cart9', 'aaaabbbbccccddddeeeeffff22', '${key}A', 'AAAABBBBCCCCDDDDEEEEFFFF1!']) {
        final res = await send('PATCH', '/_/cart/items/7', cookie: '__Host-cart=$bad', body: '{}');
        expect(seen.single.headers.containsKey('X-Cart-Key'), isFalse, reason: bad);
        expect(setCookies(res), isEmpty, reason: bad);
      }
    });

    test('Go er cart_key bhul format hole 502, cookie na', () async {
      api['/api/cart/items'] = http.Response('{"items":[],"cart_key":"a;b"}', 200);
      final res = await send('POST', '/_/cart/items', body: '{}');
      expect(res.statusCode, 502);
      expect(setCookies(res), isEmpty);
    });

    test('Go er error (409 out_of_stock) hubohu, cookie abar set na', () async {
      const err = '{"error":{"code":"out_of_stock","message":"","details":{"variant_id":5,"max_quantity":3}}}';
      api['/api/cart/items'] = http.Response(err, 409, headers: {'content-type': 'application/json'});
      final res = await send(
        'POST',
        '/_/cart/items',
        cookie: '__Host-cart=$key',
        body: '{"variant_id":5,"quantity":9}',
      );
      expect(res.statusCode, 409);
      expect(await res.readAsString(), err);
      expect(setCookies(res), isEmpty);
    });

    test('JS chhara add form: number field JSON e number, 303 /cart (bhasha shoho), notun cookie', () async {
      api['/api/cart/items'] = http.Response('{"items":[],"cart_key":"$newKey"}', 200);
      for (final (referer, to) in [
        ('https://shop.example.com/p/lal-chal', '/cart'),
        ('https://shop.example.com/en/p/lal-chal', '/en/cart'),
      ]) {
        final res = await send('POST', '/_/cart/items', form: 'variant_id=5&quantity=2', referer: referer);
        expect(jsonDecode(seen.single.body), {'variant_id': 5, 'quantity': 2});
        expect(res.statusCode, 303);
        expect(res.headers['location'], to);
        expect(setCookies(res), [cartCookie(newKey)]);
        expect(res.headers['cache-control'], 'private, no-store');
      }
    });

    test('JS chhara qty form: POST /_/cart/items/{id} → PATCH, 303 /cart', () async {
      final res = await send(
        'POST',
        '/_/cart/items/7',
        cookie: '__Host-cart=$key',
        form: 'quantity=3',
        referer: 'https://shop.example.com/cart',
      );
      final r = seen.single;
      expect(r.method, 'PATCH');
      expect(r.url.path, '/api/cart/items/7');
      expect(jsonDecode(r.body), {'quantity': 3});
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/cart');
      expect(setCookies(res), [cartCookie(key)]);
    });

    test('JS chhara bad form: POST /_/cart/items/{id}/delete → DELETE, 303 /cart', () async {
      final res = await send(
        'POST',
        '/_/cart/items/7/delete',
        cookie: '__Host-cart=$key',
        form: '',
        referer: 'https://shop.example.com/en/cart',
      );
      final r = seen.single;
      expect(r.method, 'DELETE');
      expect(r.url.path, '/api/cart/items/7');
      expect(r.headers['X-Cart-Key'], key);
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/en/cart');
    });

    test('JS chhara form e error: 303 form er page e ?error=<code>', () async {
      api['/api/cart/items'] = http.Response('{"error":{"code":"out_of_stock","message":""}}', 409);
      final res = await send(
        'POST',
        '/_/cart/items',
        form: 'variant_id=5&quantity=99',
        referer: 'https://shop.example.com/p/lal-chal',
      );
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/p/lal-chal?error=out_of_stock');
      expect(setCookies(res), isEmpty);
    });

    test('wishlist: POST/DELETE /_/wishlist/{id} → /api/wishlist/{id}, Bearer', () async {
      for (final m in ['POST', 'DELETE']) {
        api['/api/wishlist/5'] = http.Response('{"items":[]}', 200, headers: {'content-type': 'application/json'});
        final res = await send(m, '/_/wishlist/5', cookie: '__Host-sid=TOK2ABC');
        final r = seen.single;
        expect(r.method, m);
        expect(r.url.path, '/api/wishlist/5');
        expect(r.headers['Authorization'], 'Bearer TOK2ABC');
        expect(res.statusCode, 200);
        expect(await res.readAsString(), '{"items":[]}');
        // Cookie nai tai BFF `no-store`; `__Host-sid` er jonno `cacheControl` `private, no-store` kore (cache_control_test).
        expect(res.headers['cache-control'], 'no-store');
      }
    });

    for (final (m, p) in [
      ('POST', '/_/cart/items'),
      ('PATCH', '/_/cart/items/7'),
      ('DELETE', '/_/cart/items/7'),
      ('POST', '/_/cart/items/7'),
      ('POST', '/_/cart/items/7/delete'),
      ('POST', '/_/wishlist/5'),
      ('DELETE', '/_/wishlist/5'),
    ]) {
      test('$m $p onno Origin e 403, Go te jay na, cookie na', () async {
        final res = await send(m, p, cookie: '__Host-cart=$key', origin: 'https://evil.example');
        expect(res.statusCode, 403);
        expect(seen, isEmpty);
        expect(setCookies(res), isEmpty);
      });
    }

    group('verify (10.12 item 2, 3)', () {
      setUp(() {
        api['/api/auth/otp/verify'] = http.Response(
          '{"token":"TOK2NEW","user":{"phone":"01712345678"},"cart_notices":[{"code":"item_dropped","name":"Chal"}]}',
          200,
          headers: {'content-type': 'application/json'},
        );
        api['/api/wishlist/merge'] = http.Response('{"items":[]}', 200);
      });

      test('200: key Go te (purono sid thakleo), __Host-cart Max-Age=0, cart_notices thake, token na', () async {
        final res = await send(
          'POST',
          '/_/auth/otp/verify',
          cookie: '__Host-sid=OLD; __Host-cart=$key',
          body: '{"phone":"01712345678","code":"123456"}',
        );
        expect(seen.single.url.path, '/api/auth/otp/verify');
        expect(seen.single.headers['X-Cart-Key'], key);
        expect(res.statusCode, 200);
        expect(setCookies(res), unorderedEquals([contains('__Host-sid=TOK2NEW;'), cartCleared]));
        final out = jsonDecode(await res.readAsString()) as Map;
        expect(out.containsKey('token'), isFalse);
        expect(out['cart_notices'], [
          {'code': 'item_dropped', 'name': 'Chal'},
        ]);
      });

      test('wl thakle notun token diye wishlist merge (valid ID), tarpor wl Max-Age=0', () async {
        final res = await send('POST', '/_/auth/otp/verify', cookie: 'wl=3,abc,5,,-1', body: '{}');
        expect(seen.map((r) => r.url.path), ['/api/auth/otp/verify', '/api/wishlist/merge']);
        final m = seen.last;
        expect(m.method, 'POST');
        expect(m.headers['Authorization'], 'Bearer TOK2NEW');
        expect(m.headers['X-Shop-Host'], 'shop.example.com');
        expect(jsonDecode(m.body), {
          'product_ids': [3, 5],
        });
        expect(res.statusCode, 200);
        expect(setCookies(res), unorderedEquals([contains('__Host-sid=TOK2NEW;'), wlCleared]));
      });

      test('wl merge fail hole login thik thake, wl thake (porer login e abar)', () async {
        api['/api/wishlist/merge'] = http.Response('{"error":{"code":"internal","message":""}}', 500);
        final res = await send('POST', '/_/auth/otp/verify', cookie: 'wl=3', body: '{}');
        expect(res.statusCode, 200);
        expect(setCookies(res), [contains('__Host-sid=TOK2NEW;')]);
      });

      test('cart ar wl cookie na thakle shudhu sid, merge call na', () async {
        final res = await send('POST', '/_/auth/otp/verify', body: '{}');
        expect(seen, hasLength(1));
        expect(setCookies(res), [contains('__Host-sid=TOK2NEW;')]);
      });

      test('verify error: cart/wl cookie thake, merge na', () async {
        api['/api/auth/otp/verify'] = http.Response('{"error":{"code":"otp_invalid","message":""}}', 400);
        final res = await send('POST', '/_/auth/otp/verify', cookie: '__Host-cart=$key; wl=3', body: '{}');
        expect(res.statusCode, 400);
        expect(seen, hasLength(1));
        expect(setCookies(res), isEmpty);
      });
    });
  });
}
