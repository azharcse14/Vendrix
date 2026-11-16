import 'package:shelf/shelf.dart';

import 'cache_control.dart';
import 'locale.dart';

const _turnstile = 'https://challenges.cloudflare.com';

// Turnstile widget thaka page (10.11 item 3, 10.13 item 2).
const _turnstilePages = {'/login', '/checkout'};

/// Security header (12.2 Website 3, 10.10 item 7): protiti response e. Inline script nai (Jaspr client
/// `<script src>`, JSON-LD execute hoy na), tai `script-src 'self'`; `style-src 'unsafe-inline'` theme er
/// `:root` `<style>` er jonno (rong/font validate kora). Turnstile shudhu `/login` ar `/checkout` e (10.11 item 3,
/// 10.13 item 2); token page e `no-referrer` ar `X-Robots-Tag: noindex` (10.1, 10.4 item 7); payment M10 e, HSTS M12 e
/// (Caddy).
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
  return (inner) => (request) async {
    // URL er token Referer e onno site e na jay (10.1).
    final token = isTokenPage(request.url.path);
    return (await inner(request)).change(
      headers: {
        'x-content-type-options': 'nosniff',
        'referrer-policy': token ? 'no-referrer' : 'strict-origin-when-cross-origin',
        if (token) 'x-robots-tag': 'noindex',
        // dart:io er default `SAMEORIGIN` er bodole; CSP er frame-ancestors er sathe mile.
        'x-frame-options': 'DENY',
        'content-security-policy': _turnstilePages.contains(localeOf('/${request.url.path}').path) ? login : plain,
      },
    );
  };
}
