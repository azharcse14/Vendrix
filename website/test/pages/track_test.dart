import 'dart:convert';

import 'package:html/dom.dart' as dom;
import 'package:html/parser.dart' show parse;
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';
import 'package:shelf/shelf.dart' show Pipeline;
import 'package:test/test.dart';
import 'package:website/main.server.options.dart';
import 'package:website/services/cache_control.dart';
import 'package:website/services/config.dart';
import 'package:website/services/render.dart';
import 'package:website/services/settings_cache.dart';

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  final cfg = Config.fromEnv({
    'API_BASE_URL': 'http://api.test',
    'JASPR_INTERNAL_SECRET': 's3cret',
    'IMAGE_BASE_URL': 'https://img.test',
    'IMAGE_RESIZE': 'none',
    'TURNSTILE_SITE_KEY': '1x00000000000000000000AA',
  });
  const t = Translator('bn');
  const json = {'content-type': 'application/json; charset=utf-8'};

  late List<http.Request> seen;
  late http.Response Function() goResponse;
  final api = MockClient((r) async {
    seen.add(r);
    if (r.url.path == '/api/orders/track') return goResponse();
    return http.Response('{"error":{"code":"not_found","message":""}}', 404, headers: json);
  });

  Future<ShopSettings> shop(String _) async => const ShopSettings(
    PublicSettings({
      'locales': {
        'enabled': ['bn', 'en'],
      },
    }),
    1,
    {},
  );

  Future<(Response, String, dom.Document)> send(
    String method,
    String path, {
    String body = '',
    String origin = 'https://shop.test',
  }) async {
    seen = [];
    final handler = const Pipeline()
        .addMiddleware(cacheControl())
        .addHandler(serveApp((r, render) => renderPage(r, render, shop, cfg: cfg, client: api, log: (_) {})));
    final res = await handler(
      Request(
        method,
        Uri.parse('http://shop.test$path'),
        headers: {
          'host': 'shop.test',
          'x-forwarded-proto': 'https',
          'x-real-ip': '203.0.113.9',
          if (method == 'POST') ...{'origin': origin, 'content-type': 'application/x-www-form-urlencoded'},
        },
        body: body,
      ),
    );
    final html = await res.readAsString();
    return (res, html, parse(html));
  }

  setUp(
    () => goResponse = () => http.Response(
      jsonEncode({
        'order_number': 'VX-100001',
        'display_status': 'confirmed',
        'placed_at': '2026-11-16T04:00:00Z',
        'history': [
          {'status': 'pending', 'at': '2026-11-16T04:00:00Z'},
          {'status': 'confirmed', 'at': '2026-11-16T05:00:00Z'},
        ],
        'tracking': null,
      }),
      200,
      headers: json,
    ),
  );

  group('/track (10.1, 10.13 item 6: SSR POST, BFF na)', () {
    test('GET: JS chhara form nijer page e, number + tel phone, noindex, private no-store, API call na', () async {
      final (res, html, doc) = await send('GET', '/track');
      expect(res.statusCode, 200);
      expect(res.headers['cache-control'], 'private, no-store');
      expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex');
      expect(doc.querySelector('h1')!.text, t.t('track.title'));
      final form = doc.querySelector('form#track')!;
      expect(form.attributes['method'], 'post');
      expect(form.attributes['action'], '/track');
      final phone = form.querySelector('input[name="phone"]')!;
      expect(phone.attributes['type'], 'tel');
      expect(phone.attributes['autocomplete'], 'tel');
      expect(phone.parent!.localName, 'label');
      expect(form.querySelector('input[name="number"]')!.parent!.localName, 'label');
      expect(html, isNot(contains('<!--@')));
      expect(seen, isEmpty);
    });

    test('POST: Go te JSON body (phone shudhu body te) + X-Client-IP; status, tarikh, itihash; link na', () async {
      final (res, _, doc) = await send('POST', '/track', body: 'number=vx-100001&phone=01712345678');
      expect(res.statusCode, 200);
      expect(res.headers['cache-control'], 'private, no-store');
      final r = seen.single;
      expect(r.method, 'POST');
      expect(r.url.toString(), 'http://api.test/api/orders/track');
      expect(r.url.query, isEmpty);
      expect(jsonDecode(r.body), {'number': 'vx-100001', 'phone': '01712345678'});
      expect(r.headers['X-Client-IP'], '203.0.113.9');
      expect(r.headers['X-Internal-Auth'], 's3cret');
      expect(r.headers['X-Shop-Host'], 'shop.test');
      final result = doc.querySelector('#track-result')!;
      expect(result.attributes['role'], 'status');
      expect(result.text, contains('VX-100001'));
      expect(result.text, contains(t.t('order.status.confirmed')));
      expect(result.text, contains(t.t('order.status.pending')));
      expect(result.text, contains(t.t('track.history')));
      expect(doc.querySelector('a[href^="/order/"]'), isNull);
      // Number abar bhora, phone khali.
      expect(doc.querySelector('input[name="number"]')!.attributes['value'], 'vx-100001');
      expect(doc.querySelector('input[name="phone"]')!.attributes['value'] ?? '', isEmpty);
    });

    test('English: /en/track POST e Accept-Language en, lekha English', () async {
      final (res, _, doc) = await send('POST', '/en/track', body: 'number=VX-100001&phone=01712345678');
      expect(res.statusCode, 200);
      expect(seen.single.headers['Accept-Language'], 'en');
      expect(doc.querySelector('#track-result')!.text, contains('Confirmed'));
    });

    test('404: track.not_found lekha, status 404', () async {
      goResponse = () => http.Response('{"error":{"code":"not_found","message":""}}', 404, headers: json);
      final (res, _, doc) = await send('POST', '/track', body: 'number=X&phone=01712345678');
      expect(res.statusCode, 404);
      expect(doc.querySelector('[role="alert"]')!.text, t.t('track.not_found'));
      expect(doc.querySelector('form#track'), isNotNull);
    });

    test('429: status ar Retry-After hubohu, lekha rate_limited', () async {
      goResponse = () => http.Response(
        '{"error":{"code":"rate_limited","message":""}}',
        429,
        headers: {...json, 'retry-after': '42'},
      );
      final (res, _, doc) = await send('POST', '/track', body: 'number=X&phone=01712345678');
      expect(res.statusCode, 429);
      expect(res.headers['retry-after'], '42');
      expect(doc.querySelector('[role="alert"]')!.text, t.t('error.rate_limited'));
    });

    test('400 validation_failed: field er bhul (phone)', () async {
      goResponse = () => http.Response(
        '{"error":{"code":"validation_failed","message":"","fields":{"phone":"invalid_phone"}}}',
        400,
        headers: json,
      );
      final (res, _, doc) = await send('POST', '/track', body: 'number=X&phone=123');
      expect(res.statusCode, 400);
      final phone = doc.querySelector('input[name="phone"]')!;
      expect(phone.attributes['aria-invalid'], 'true');
      expect(doc.querySelector('#track-phone-error')!.text, t.t('error.invalid_phone'));
      expect(phone.attributes['aria-describedby'], 'track-phone-error');
    });

    test('Go down (5xx): 503 error page', () async {
      goResponse = () => http.Response('', 502);
      final (res, _, _) = await send('POST', '/track', body: 'number=X&phone=01712345678');
      expect(res.statusCode, 503);
    });

    test('onno Origin e 403, Go te jay na (CSRF, 10.9 niyom 1)', () async {
      final (res, _, _) = await send('POST', '/track', body: 'number=X&phone=1', origin: 'https://evil.example');
      expect(res.statusCode, 403);
      expect(seen, isEmpty);
    });

    test('boro body 413, Go te jay na', () async {
      final (res, _, _) = await send('POST', '/track', body: 'number=${'x' * (1 << 20)}&phone=1');
      expect(res.statusCode, 413);
      expect(seen, isEmpty);
    });
  });
}
