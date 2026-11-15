import 'package:shelf/shelf.dart';
import 'package:test/test.dart';
import 'package:website/services/security_headers.dart';

void main() {
  group('Security header ar CSP (12.2, 10.10)', () {
    test('protiti response e nosniff, referrer, frame bondho, CSP (chobir origin shoho)', () async {
      final handler = securityHeaders(Uri.parse('https://img.example.com/cdn/x'))(
        (_) => Response.notFound('', headers: {'content-type': 'text/html'}),
      );
      final h = (await handler(Request('GET', Uri.parse('http://shop.test/p/x')))).headers;
      expect(h['x-content-type-options'], 'nosniff');
      expect(h['referrer-policy'], 'strict-origin-when-cross-origin');
      expect(h['x-frame-options'], 'DENY');
      final csp = {
        for (final d in h['content-security-policy']!.split(';').map((s) => s.trim()))
          d.split(' ').first: d.split(' ').skip(1).join(' '),
      };
      expect(csp, {
        'default-src': "'self'",
        'script-src': "'self'",
        'style-src': "'self' 'unsafe-inline'",
        'img-src': "'self' https://img.example.com data:",
        'font-src': "'self'",
        'connect-src': "'self'",
        'base-uri': "'none'",
        'form-action': "'self'",
        'frame-ancestors': "'none'",
      });
    });

    test('Turnstile shudhu /login ar /en/login e: script-src ar frame-src (10.11 item 3)', () async {
      final handler = securityHeaders(Uri.parse('https://img.example.com'))((_) => Response.ok(''));
      Future<Map<String, String>> csp(String path) async {
        final h = (await handler(Request('GET', Uri.parse('http://shop.test$path')))).headers;
        return {
          for (final d in h['content-security-policy']!.split(';').map((s) => s.trim()))
            d.split(' ').first: d.split(' ').skip(1).join(' '),
        };
      }

      for (final path in ['/login', '/en/login', '/login?next=/account']) {
        final c = await csp(path);
        expect(c['script-src'], "'self' https://challenges.cloudflare.com", reason: path);
        expect(c['frame-src'], 'https://challenges.cloudflare.com', reason: path);
        expect(c['frame-ancestors'], "'none'", reason: path);
      }
      for (final path in ['/', '/account', '/p/login', '/login/x', '/_/auth/otp/send']) {
        final c = await csp(path);
        expect(c['script-src'], "'self'", reason: path);
        expect(c.containsKey('frame-src'), isFalse, reason: path);
      }
    });
  });
}
