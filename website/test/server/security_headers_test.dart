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
  });
}
