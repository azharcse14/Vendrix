import 'package:shelf/shelf.dart';
import 'package:test/test.dart';
import 'package:website/services/cache_control.dart';

void main() {
  Future<String?> run(String path, {String? cookie, Response? response}) async {
    final handler = cacheControl()((_) => response ?? Response.ok('<html>'));
    final res = await handler(
      Request('GET', Uri.parse('http://shop.example.com$path'), headers: {'cookie': ?cookie}),
    );
    return res.headers['cache-control'];
  }

  group('Cache-Control (10.3, 14.2 test 24)', () {
    test('cookie chhara public page: public, max-age=60', () async {
      expect(await run('/p/lal-chal'), 'public, max-age=60');
      expect(await run('/en/c/chal', cookie: 'lang=en; wl=1,2'), 'public, max-age=60');
    });

    for (final cookie in ['__Host-sid=abc', 'lang=bn; __Host-cart=k1', 'preview=tok']) {
      test('$cookie → private, no-store', () async {
        expect(await run('/p/lal-chal', cookie: cookie), 'private, no-store');
      });
    }

    test('cookie thakle page er nijer header o bodle private, no-store', () async {
      final res = Response.ok('', headers: {'cache-control': 'public, max-age=60'});
      expect(await run('/', cookie: '__Host-sid=abc', response: res), 'private, no-store');
    });

    test('naam er ongsho mile na: x__Host-sid public', () async {
      expect(await run('/', cookie: 'x__Host-sid=abc; previewer=1'), 'public, max-age=60');
    });

    test('BFF /_/... no-store', () async {
      expect(await run('/_/auth/otp/send'), 'no-store');
    });

    test('page nije dile (503 error page) thake', () async {
      final res = Response(503, headers: {'cache-control': 'no-store'});
      expect(await run('/', response: res), 'no-store');
    });

    test('2xx na hole (404) no-store', () async {
      expect(await run('/nai', response: Response.notFound('')), 'no-store');
    });

    test('Set-Cookie thakle private, no-store', () async {
      final res = Response.ok('', headers: {'set-cookie': 'lang=en'});
      expect(await run('/', response: res), 'private, no-store');
    });
  });
}
