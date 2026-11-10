import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:test/test.dart';
import 'package:website/services/api.dart';
import 'package:website/services/config.dart';

void main() {
  final cfg = Config.fromEnv({
    'API_BASE_URL': 'http://127.0.0.1:8090',
    'JASPR_INTERNAL_SECRET': 's3cret',
    'IMAGE_BASE_URL': 'https://img.example.com',
    'IMAGE_RESIZE': 'cloudflare',
  });

  group('API header (3.2, 10.3)', () {
    test('Host → X-Shop-Host; X-Real-IP na thakle IP ar secret na', () {
      expect(apiHeaders({'host': 'sobujbazar.localhost:8081'}, cfg), {'X-Shop-Host': 'sobujbazar.localhost:8081'});
    });

    test('Caddy er X-Real-IP → X-Client-IP + X-Internal-Auth', () {
      expect(apiHeaders({'host': 'shop.example.com', 'x-real-ip': '203.0.113.7'}, cfg), {
        'X-Shop-Host': 'shop.example.com',
        'X-Client-IP': '203.0.113.7',
        'X-Internal-Auth': 's3cret',
      });
    });

    test('browser er Accept-Language na, URL er bhasha jay', () async {
      late http.BaseRequest seen;
      final client = MockClient((req) async {
        seen = req;
        return http.Response('{}', 200);
      });
      await apiFor({'host': 'shop.example.com', 'accept-language': 'en-US'}, 'bn', cfg, client).get('/api/settings');
      expect(seen.url.toString(), 'http://127.0.0.1:8090/api/settings');
      expect(seen.headers['Accept-Language'], 'bn');
      expect(seen.headers['X-Shop-Host'], 'shop.example.com');
    });
  });

  group('Config (13.5)', () {
    test('nai ba bhul env e FormatException, naam shoho', () {
      expect(
        () => Config.fromEnv({}),
        throwsA(isA<FormatException>().having((e) => e.message, 'message', contains('API_BASE_URL'))),
      );
      expect(
        () => Config.fromEnv({
          'API_BASE_URL': 'http://127.0.0.1:8090',
          'JASPR_INTERNAL_SECRET': 'x',
          'IMAGE_BASE_URL': 'https://img.example.com',
          'IMAGE_RESIZE': 'imgix',
        }),
        throwsA(isA<FormatException>().having((e) => e.message, 'message', contains('IMAGE_RESIZE'))),
      );
    });

    test('IMAGE_RESIZE: cloudflare → true', () {
      expect(cfg.imageResize, isTrue);
      expect(cfg.imageBaseUrl.toString(), 'https://img.example.com');
    });
  });
}
