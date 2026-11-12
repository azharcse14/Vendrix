import 'package:http/http.dart';
import 'package:http/testing.dart';
import 'package:shared/shared.dart';
import 'package:test/test.dart';
import 'package:website/components/site.dart';
import 'package:website/pages/status.dart';
import 'package:website/services/image.dart';
import 'package:website/services/render.dart';
import 'package:website/services/settings_cache.dart';

void main() {
  ShopSettings shop({List<String> locales = const ['bn', 'en']}) => ShopSettings(
    PublicSettings.fromJson({
      'locales': {'enabled': locales},
    }),
    1,
    const {
      'bn': {},
      'en': {'nav.home': 'Start'},
    },
  );

  late Site site;
  Future<({int statusCode, Map<String, String> headers})> get(
    String path,
    Future<ShopSettings> Function(String host) settings,
  ) async {
    final p = await resolvePage(
      'shop.test',
      path,
      settings,
      catalogFor: (locale) =>
          CatalogRepository(ApiClient(Uri.parse('http://api.test'), MockClient((_) async => Response('', 500)))),
      images: ImageUrls(Uri.parse('http://img.test'), resize: false),
      log: (_) {},
    );
    site = p.site;
    return (statusCode: p.status, headers: p.headers);
  }

  group('Page status (10.10 item 10)', () {
    test('shop thakle 200, bhasha URL theke, override shoho', () async {
      final res = await get('/en/c/rice', (_) async => shop());
      expect(res.statusCode, 200);
      expect(site.locale, 'en');
      expect(site.t('nav.home'), 'Start');
      await get('/', (_) async => shop());
      expect(site.locale, 'bn');
    });

    test('locales.enabled e en na thakle /en/* 404', () async {
      final res = await get('/en', (_) async => shop(locales: ['bn']));
      expect(res.statusCode, 404);
      expect(site.child, isA<StatusPage>().having((p) => p.kind, 'kind', StatusKind.notFound));
    });

    test('shop nai → 404', () async {
      final res = await get('/', (_) async => throw const ApiError(404, 'shop_not_found'));
      expect(res.statusCode, 404);
    });

    test('shop bondho → 503, Retry-After 3600, closed page', () async {
      final res = await get('/', (_) async => throw const ApiError(503, 'shop_suspended'));
      expect(res.statusCode, 503);
      expect(res.headers['retry-after'], '3600');
      expect(site.child, isA<StatusPage>().having((p) => p.kind, 'kind', StatusKind.closed));
    });

    test('API down ba onno bhul → 503 error page', () async {
      for (final err in [const ApiError(0, 'network_error'), StateError('x')]) {
        final res = await get('/', (_) async => throw err);
        expect(res.statusCode, 503);
        expect(res.headers['retry-after'], isNull);
        expect(site.child, isA<StatusPage>().having((p) => p.kind, 'kind', StatusKind.error));
      }
    });
  });
}
