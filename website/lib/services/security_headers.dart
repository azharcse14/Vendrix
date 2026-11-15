import 'package:shelf/shelf.dart';

import 'locale.dart';

const _turnstile = 'https://challenges.cloudflare.com';

/// Security header (12.2 Website 3, 10.10 item 7): protiti response e. Inline script nai (Jaspr client
/// `<script src>`, JSON-LD execute hoy na), tai `script-src 'self'`; `style-src 'unsafe-inline'` theme er
/// `:root` `<style>` er jonno (rong/font validate kora). Turnstile shudhu `/login` e (10.11 item 3); payment M10 e,
/// HSTS M12 e (Caddy).
Middleware securityHeaders(Uri imageBase) {
  String csp(bool turnstile) => [
    "default-src 'self'",
    turnstile ? "script-src 'self' $_turnstile" : "script-src 'self'",
    if (turnstile) 'frame-src $_turnstile',
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' ${imageBase.origin} data:",
    "font-src 'self'",
    "connect-src 'self'",
    "base-uri 'none'",
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join('; ');
  final plain = csp(false), login = csp(true);
  return (inner) =>
      (request) async => (await inner(request)).change(
        headers: {
          'x-content-type-options': 'nosniff',
          'referrer-policy': 'strict-origin-when-cross-origin',
          // dart:io er default `SAMEORIGIN` er bodole; CSP er frame-ancestors er sathe mile.
          'x-frame-options': 'DENY',
          'content-security-policy': localeOf('/${request.url.path}').path == '/login' ? login : plain,
        },
      );
}
