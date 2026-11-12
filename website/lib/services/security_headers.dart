import 'package:shelf/shelf.dart';

/// Security header (12.2 Website 3, 10.10 item 7): protiti response e. Inline script nai (Jaspr client
/// `<script src>`, JSON-LD execute hoy na), tai `script-src 'self'`; `style-src 'unsafe-inline'` theme er
/// `:root` `<style>` er jonno (rong/font validate kora). Turnstile/payment domain M4/M10 e, HSTS M12 e (Caddy).
Middleware securityHeaders(Uri imageBase) {
  final csp = [
    "default-src 'self'",
    "script-src 'self'",
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' ${imageBase.origin} data:",
    "font-src 'self'",
    "connect-src 'self'",
    "base-uri 'none'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join('; ');
  return (inner) =>
      (request) async => (await inner(request)).change(
        headers: {
          'x-content-type-options': 'nosniff',
          'referrer-policy': 'strict-origin-when-cross-origin',
          // dart:io er default `SAMEORIGIN` er bodole; CSP er frame-ancestors er sathe mile.
          'x-frame-options': 'DENY',
          'content-security-policy': csp,
        },
      );
}
