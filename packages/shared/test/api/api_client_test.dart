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
}
