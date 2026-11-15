import 'dart:async';
import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';
import 'package:test/test.dart';

http.Response json(
  Object body,
  int status, [
  Map<String, String> headers = const {},
]) => http.Response(
  jsonEncode(body),
  status,
  headers: {'content-type': 'application/json; charset=utf-8', ...headers},
);

void main() {
  group('ApiClient (7.1, v1.12)', () {
    late http.Request last;
    ApiClient client(
      Future<http.Response> Function(http.Request) h, {
      Duration? timeout,
    }) => ApiClient(
      Uri.parse('http://api.test'),
      MockClient((r) {
        last = r;
        return h(r);
      }),
      locale: 'en',
      headers: {'X-Shop-Host': 'sobujbazar.com'},
      timeout: timeout ?? const Duration(seconds: 15),
    );

    test('header, query, 2xx JSON', () async {
      final c = client((_) async => json({'ok': true}, 200));
      expect(await c.get('/api/products', query: {'q': 'চাল', 'page': '2'}), {
        'ok': true,
      });
      expect(
        last.url.toString(),
        'http://api.test/api/products?q=%E0%A6%9A%E0%A6%BE%E0%A6%B2&page=2',
      );
      expect(last.headers['Accept-Language'], 'en');
      expect(last.headers['X-Shop-Host'], 'sobujbazar.com');
    });
    test('body JSON, 204 e null', () async {
      final c = client((r) async => http.Response('', 204));
      expect(
        await c.send(
          'PATCH',
          '/api/admin/products/1',
          body: {'is_active': false},
        ),
        isNull,
      );
      expect(last.method, 'PATCH');
      expect(last.headers['Content-Type'], startsWith('application/json'));
      expect(jsonDecode(last.body), {'is_active': false});
    });
    test('7.1 error body → ApiError', () async {
      final c = client(
        (_) async => json({
          'error': {
            'code': 'validation_failed',
            'message': 'Some information is invalid',
            'fields': {'slug': 'invalid'},
            'details': {'id': 3},
          },
        }, 400),
      );
      final e = await c
          .get('/x')
          .then<ApiError?>((_) => null, onError: (Object e) => e as ApiError);
      expect(
        [e!.status, e.code, e.message, e.fields, e.details],
        [
          400,
          'validation_failed',
          'Some information is invalid',
          {'slug': 'invalid'},
          {'id': 3},
        ],
      );
    });
    test('bhul shape er error body o ApiError', () async {
      final c = client(
        (_) async => json({
          'error': {
            'code': 1,
            'fields': {
              'a': [1],
              'b': 'invalid',
            },
          },
        }, 400),
      );
      await expectLater(
        c.get('/x'),
        throwsA(
          isA<ApiError>().having((e) => [e.code, e.fields], 's', [
            'internal',
            {'b': 'invalid'},
          ]),
        ),
      );
      final empty = client((_) async => http.Response('', 201));
      expect(await empty.send('POST', '/x', body: <String, Object>{}), isNull);
    });
    test('429 Retry-After', () async {
      final c = client(
        (_) async => json(
          {
            'error': {'code': 'rate_limited', 'message': 'x'},
          },
          429,
          {'retry-after': '50'},
        ),
      );
      await expectLater(
        c.get('/x'),
        throwsA(
          isA<ApiError>().having(
            (e) => e.retryAfter,
            'retryAfter',
            const Duration(seconds: 50),
          ),
        ),
      );
    });
    test('JSON na (HTML 502) → internal, asol status', () async {
      final c = client(
        (_) async => http.Response('<html>Bad gateway</html>', 502),
      );
      await expectLater(
        c.get('/x'),
        throwsA(
          isA<ApiError>().having((e) => [e.status, e.code], 's', [
            502,
            'internal',
          ]),
        ),
      );
      final ok = client((_) async => http.Response('not json', 200));
      await expectLater(
        ok.get('/x'),
        throwsA(
          isA<ApiError>().having((e) => [e.status, e.code], 's', [
            200,
            'internal',
          ]),
        ),
      );
    });
    test('network fail → network_error, timeout → timeout', () async {
      final c = client((_) async => throw http.ClientException('offline'));
      await expectLater(
        c.get('/x'),
        throwsA(
          isA<ApiError>().having((e) => [e.status, e.code], 's', [
            0,
            'network_error',
          ]),
        ),
      );
      final slow = client(
        (_) => Completer<http.Response>().future,
        timeout: const Duration(milliseconds: 10),
      );
      await expectLater(
        slow.get('/x'),
        throwsA(
          isA<ApiError>().having((e) => [e.status, e.code], 's', [
            0,
            'timeout',
          ]),
        ),
      );
    });
  });

  group('Repository (7.2)', () {
    test('CatalogRepository query ar model', () async {
      late Uri url;
      final api = ApiClient(
        Uri.parse('http://api.test'),
        MockClient((r) async {
          url = r.url;
          if (r.url.path == '/api/categories') {
            return json([
              {'id': 1, 'slug': 'c', 'name': 'চাল', 'image_key': null},
            ], 200);
          }
          if (r.url.path == '/api/policies/terms') {
            return json({'kind': 'terms', 'body': 'শর্ত'}, 200);
          }
          if (r.url.path == '/api/products/lal-chal') {
            return json({
              'id': 1,
              'slug': 'lal-chal',
              'name': 'x',
              'description': '',
              'unit': 'kg',
              'variable_weight': true,
              'category': null,
              'attributes': <String, Object>{},
              'seo': <String, Object>{},
              'rating': 0,
              'review_count': 0,
              'variants': <Object>[],
              'images': <Object>[],
            }, 200);
          }
          return json({
            'items': <Object>[],
            'page': 1,
            'limit': 24,
            'total': 0,
          }, 200);
        }),
      );
      final repo = CatalogRepository(api);
      final p = await repo.products(
        q: 'চাল',
        category: 'chal',
        minPrice: 100,
        inStock: true,
        sort: 'price_asc',
        page: 2,
      );
      expect(p.total, 0);
      expect(url.queryParameters, {
        'q': 'চাল',
        'category': 'chal',
        'min_price': '100',
        'in_stock': 'true',
        'sort': 'price_asc',
        'page': '2',
      });
      expect((await repo.categories()).single.name, 'চাল');
      expect((await repo.product('lal-chal')).variableWeight, isTrue);
      expect(await repo.policy('terms'), 'শর্ত');
    });
    test('SettingsRepository', () async {
      final api = ApiClient(
        Uri.parse('http://api.test'),
        MockClient(
          (r) async => switch (r.url.path) {
            '/api/settings' => json({
              'locales': {'default': 'bn'},
            }, 200),
            '/api/settings/version' => json({'version': 7}, 200),
            _ => json({'cart.add': 'Add'}, 200),
          },
        ),
      );
      final repo = SettingsRepository(api);
      expect((await repo.settings()).defaultLocale, 'bn');
      expect(await repo.version(), 7);
      expect(await repo.translations('en'), {'cart.add': 'Add'});
    });
  });

  group('Account (7.3, 10.11 item 4)', () {
    test('token thakle Authorization: Bearer; me() → User', () async {
      late http.Request seen;
      final api = ApiClient(
        Uri.parse('http://api.test'),
        MockClient((r) async {
          seen = r;
          return json({
            'phone': '01712345678',
            'name': 'রহিম',
            'email': null,
            'locale': 'bn',
          }, 200);
        }),
        token: 'TOK2ABC',
      );
      final u = await AccountRepository(api).me();
      expect(seen.url.path, '/api/me');
      expect(seen.headers['Authorization'], 'Bearer TOK2ABC');
      expect(
        [u.phone, u.name, u.email, u.locale],
        ['01712345678', 'রহিম', null, 'bn'],
      );
    });
    test(
      'token na thakle Authorization nai; shape bhul hole FormatException',
      () async {
        late http.Request seen;
        final api = ApiClient(
          Uri.parse('http://api.test'),
          MockClient((r) async {
            seen = r;
            return json({'phone': 1}, 200);
          }),
        );
        await expectLater(AccountRepository(api).me(), throwsFormatException);
        expect(seen.headers.containsKey('Authorization'), isFalse);
      },
    );
  });

  group('Cart ar wishlist (7.4, 8.4, M5)', () {
    final cartJson = {
      'items': [
        {
          'variant_id': 7,
          'product_slug': 'lal-chal',
          'name': 'লাল চাল',
          'variant_label': '৫ কেজি',
          'image': 'p/1.jpg',
          'unit_price': 45000,
          'compare_at_price': null,
          'variable_weight': false,
          'quantity': 2,
          'max_quantity': 50,
          'in_stock': true,
          'line_total': 90000,
        },
        {
          'variant_id': 9,
          'product_slug': 'ilish',
          'name': 'ইলিশ',
          'variant_label': '',
          'image': null,
          'unit_price': 120000,
          'compare_at_price': 150000,
          'variable_weight': true,
          'quantity': 1,
          'max_quantity': 0,
          'in_stock': false,
          'line_total': 0,
        },
      ],
      'count': 2,
      'subtotal': 90000,
      'notices': [
        {'code': 'item_removed', 'name': 'মধু'},
      ],
    };
    final card = {
      'id': 3,
      'slug': 'modhu',
      'name': 'মধু',
      'unit': 'pcs',
      'variable_weight': false,
      'price': 80000,
      'compare_at_price': null,
      'in_stock': true,
      'rating': 0,
      'review_count': 0,
      'image': null,
    };
    late List<http.Request> seen;
    ApiClient api(
      Object Function(http.Request) body, {
      String? token,
      String? cartKey,
      int status = 200,
    }) {
      seen = [];
      return ApiClient(
        Uri.parse('http://api.test'),
        MockClient((r) async {
          seen.add(r);
          return json(body(r), status);
        }),
        token: token,
        cartKey: cartKey,
      );
    }

    test('cartKey thakle X-Cart-Key, na thakle header nai', () async {
      final repo = CartRepository(
        api((_) => cartJson, cartKey: 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'),
      );
      await repo.cart();
      expect(seen.single.headers['X-Cart-Key'], 'ABCDEFGHIJKLMNOPQRSTUVWXYZ');
      await CartRepository(api((_) => cartJson)).cart();
      expect(seen.single.headers.containsKey('X-Cart-Key'), isFalse);
    });
    test(
      'cart() → Cart, CartItem, CartNotice; cart_key na thakle null',
      () async {
        final c = await CartRepository(api((_) => cartJson)).cart();
        expect(seen.single.url.path, '/api/cart');
        expect([c.count, c.subtotal, c.cartKey], [2, 90000, null]);
        final a = c.items.first, b = c.items.last;
        expect(
          [
            a.variantId,
            a.productSlug,
            a.name,
            a.variantLabel,
            a.image,
            a.unitPrice,
            a.compareAtPrice,
            a.variableWeight,
            a.quantity,
            a.maxQuantity,
            a.inStock,
            a.lineTotal,
          ],
          [
            7,
            'lal-chal',
            'লাল চাল',
            '৫ কেজি',
            'p/1.jpg',
            45000,
            null,
            false,
            2,
            50,
            true,
            90000,
          ],
        );
        expect(
          [b.image, b.compareAtPrice, b.variableWeight, b.inStock],
          [null, 150000, true, false],
        );
        expect(
          [c.notices.single.code, c.notices.single.name],
          ['item_removed', 'মধু'],
        );
      },
    );
    test(
      'add, setQuantity, remove: method, path, body; notun cart e cart_key',
      () async {
        final repo = CartRepository(
          api(
            (r) => {...cartJson, if (r.method == 'POST') 'cart_key': 'NEWKEY'},
          ),
        );
        final added = await repo.add(7, 2);
        expect(added.cartKey, 'NEWKEY');
        expect(
          [seen.last.method, seen.last.url.path],
          ['POST', '/api/cart/items'],
        );
        expect(jsonDecode(seen.last.body), {'variant_id': 7, 'quantity': 2});
        await repo.setQuantity(7, 3);
        expect(
          [seen.last.method, seen.last.url.path],
          ['PATCH', '/api/cart/items/7'],
        );
        expect(jsonDecode(seen.last.body), {'quantity': 3});
        final removed = await repo.remove(7);
        expect(
          [seen.last.method, seen.last.url.path],
          ['DELETE', '/api/cart/items/7'],
        );
        expect(removed.count, 2);
      },
    );
    test(
      'out_of_stock e details.max_quantity; cart_full, wishlist_full code',
      () async {
        Future<ApiError> err(Future<Object?> f) => f.then<ApiError>(
          (_) => fail('error chai'),
          onError: (Object e) => e as ApiError,
        );
        final oos = await err(
          CartRepository(
            api(
              (_) => {
                'error': {
                  'code': 'out_of_stock',
                  'message': 'x',
                  'details': {'variant_id': 7, 'max_quantity': 3},
                },
              },
              status: 409,
            ),
          ).add(7, 5),
        );
        expect(
          [oos.status, oos.code, oos.details['max_quantity']],
          [409, 'out_of_stock', 3],
        );
        final full = await err(
          CartRepository(
            api(
              (_) => {
                'error': {'code': 'cart_full', 'message': 'x'},
              },
              status: 409,
            ),
          ).add(8, 1),
        );
        expect(full.code, 'cart_full');
        final wl = await err(
          WishlistRepository(
            api(
              (_) => {
                'error': {'code': 'wishlist_full', 'message': 'x'},
              },
              status: 409,
              token: 'T',
            ),
          ).add(3),
        );
        expect(wl.code, 'wishlist_full');
      },
    );
    test('wishlist: Bearer, route, ProductCard list, merge body', () async {
      final repo = WishlistRepository(
        api(
          (_) => {
            'items': [card],
          },
          token: 'TOK',
        ),
      );
      final l = await repo.wishlist();
      expect(seen.last.url.path, '/api/wishlist');
      expect(seen.last.headers['Authorization'], 'Bearer TOK');
      expect([l.single.id, l.single.slug, l.single.price], [3, 'modhu', 80000]);
      await repo.add(3);
      expect(
        [seen.last.method, seen.last.url.path],
        ['POST', '/api/wishlist/3'],
      );
      await repo.remove(3);
      expect(
        [seen.last.method, seen.last.url.path],
        ['DELETE', '/api/wishlist/3'],
      );
      final m = await repo.merge([3, 4]);
      expect(
        [seen.last.method, seen.last.url.path],
        ['POST', '/api/wishlist/merge'],
      );
      expect(jsonDecode(seen.last.body), {
        'product_ids': [3, 4],
      });
      expect(m.single.name, 'মধু');
    });
    test('shape bhul hole FormatException', () async {
      await expectLater(
        CartRepository(api((_) => {...cartJson, 'subtotal': '900'})).cart(),
        throwsFormatException,
      );
    });
  });
}
