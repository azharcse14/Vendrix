import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';
import 'package:shelf/shelf.dart';
import 'package:test/test.dart';
import 'package:website/main.server.options.dart';
import 'package:website/services/config.dart';
import 'package:website/services/render.dart';
import 'package:website/services/settings_cache.dart';

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  final cfg = Config.fromEnv({
    'API_BASE_URL': 'http://api.test',
    'JASPR_INTERNAL_SECRET': 's',
    'IMAGE_BASE_URL': 'https://img.test',
    'IMAGE_RESIZE': 'none',
  });
  // Catalog API down (500): page er API bhul.
  final api = MockClient((_) async => http.Response('', 500));

  Future<Response> get(String path, Future<ShopSettings> Function(String host) settings) {
    final handler = serveApp((r, render) => renderPage(r, render, settings, cfg: cfg, client: api, log: (_) {}));
    return Future.value(handler(Request('GET', Uri.parse('http://shop.test$path'), headers: {'host': 'shop.test'})));
  }

  Future<ShopSettings> shop(String _) async => const ShopSettings(
    PublicSettings({
      'locales': {
        'enabled': ['bn', 'en'],
      },
    }),
    1,
    {},
  );

  group('renderPage (9.8, 10.10 item 10)', () {
    test('<html lang> URL er bhasha; router er 404 ar page er 503 status thake', () async {
      final en = await get('/en/nai', shop);
      expect(en.statusCode, 404);
      expect(await en.readAsString(), contains('<html lang="en"'));
      expect(await (await get('/policy/terms', shop)).readAsString(), contains('<html lang="bn"'));
      expect((await get('/p/x', shop)).statusCode, 503);
    });

    test('bondho shop: 503 + Retry-After', () async {
      final res = await get('/', (_) async => throw const ApiError(423, 'shop_suspended'));
      expect(res.statusCode, 503);
      expect(res.headers['retry-after'], '3600');
    });
  });
}
