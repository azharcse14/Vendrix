import 'dart:convert';

import 'package:html/dom.dart' as dom;
import 'package:html/parser.dart' show parse;
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';
import 'package:test/test.dart';
import 'package:website/main.server.options.dart';
import 'package:website/services/cache_control.dart';
import 'package:website/services/config.dart';
import 'package:website/services/render.dart';
import 'package:website/services/security_headers.dart';
import 'package:website/services/settings_cache.dart';

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  final cfg = Config.fromEnv({
    'API_BASE_URL': 'http://api.test',
    'JASPR_INTERNAL_SECRET': 's',
    'IMAGE_BASE_URL': 'https://img.test',
    'IMAGE_RESIZE': 'none',
    'TURNSTILE_SITE_KEY': '1x00000000000000000000AA',
  });
  const t = Translator('bn');
  const tok = 'QWERTYUIOPASDFGHJKLZXCVBNM';
  const json = {'content-type': 'application/json; charset=utf-8'};

  Map<String, dynamic> summary(String status) => {
    'order_number': 'VX-100001',
    'display_status': status,
    'placed_at': '2026-11-16T04:00:00Z',
    'customer_name': 'রহিম',
    'phone_masked': '+88017****5678',
    'district': 'Dhaka',
    'area': 'Mirpur',
    'payment_method': 'cod',
    'total': 176000,
    'items': [
      {
        'name': {'bn': 'লাল চাল', 'en': 'Red rice'},
        'variant_label': {'bn': '৫ কেজি', 'en': '5 kg'},
        'quantity': 2,
        'line_total': 170000,
      },
    ],
  };
  var status = 'pending';
  var canCancel = true;
  late List<http.Request> seen;
  final api = MockClient((r) async {
    seen.add(r);
    final auth = r.headers['Authorization'];
    if (auth != null && auth != 'Bearer GOOD') {
      return http.Response('{"error":{"code":"session_expired","message":""}}', 401, headers: json);
    }
    final Object? body = switch (r.url.path) {
      '/api/orders/$tok' => {
        ...summary(status),
        'recipient_name': 'করিম',
        'recipient_phone_masked': '+88018****1111',
        'subtotal': 170000,
        'discount': 0,
        'delivery_charge': 6000,
        'delivery_deadline': '2026-11-21T04:00:00Z',
        'can_cancel': canCancel,
      },
      '/api/confirm/$tok' => summary(status),
      '/api/me' => {'phone': '01712345678', 'name': null, 'email': null, 'locale': 'bn'},
      '/api/me/orders' => {
        'items': [
          {
            'order_number': 'VX-100002',
            'public_token': 'TOKEN2TOKEN2TOKEN2TOKEN2TO',
            'display_status': 'confirmed',
            'total': 50000,
            'placed_at': '2026-11-15T04:00:00Z',
          },
        ],
        'page': int.parse(r.url.queryParameters['page'] ?? '1'),
        'limit': 20,
        'total': 21,
      },
      _ => null,
    };
    if (body == null) return http.Response('{"error":{"code":"not_found","message":""}}', 404, headers: json);
    return http.Response(jsonEncode(body), 200, headers: json);
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

  // Asol pipeline er header middleware shoho (token page er header).
  Future<(Response, String, dom.Document)> page(String path, {String? cookie}) async {
    seen = [];
    final handler = const Pipeline()
        .addMiddleware(securityHeaders(Uri.parse('https://img.test')))
        .addMiddleware(cacheControl())
        .addHandler(serveApp((r, render) => renderPage(r, render, shop, cfg: cfg, client: api, log: (_) {})));
    final res = await handler(
      Request('GET', Uri.parse('http://shop.test$path'), headers: {'host': 'shop.test', 'cookie': ?cookie}),
    );
    final html = await res.readAsString();
    return (res, html, parse(html));
  }

  setUp(() {
    status = 'pending';
    canCancel = true;
  });

  void tokenHeaders(Response res, String html, dom.Document doc, String path) {
    expect(res.headers['referrer-policy'], 'no-referrer', reason: path);
    expect(res.headers['x-robots-tag'], 'noindex', reason: path);
    expect(res.headers['cache-control'], 'private, no-store', reason: path);
    expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex', reason: path);
    // Third-party script na (10.1 v1.5): shudhu nijer path er script.
    for (final s in doc.querySelectorAll('script[src]')) {
      expect(s.attributes['src'], startsWith('/'), reason: path);
    }
    expect(res.headers['content-security-policy'], isNot(contains('challenges.cloudflare.com')), reason: path);
  }

  group('Order page /order/{token} (10.1, 10.13 item 4, 8.19 item 6)', () {
    test('mask kora data, status, item, taka; can_cancel e cancel form; token header', () async {
      final (res, html, doc) = await page('/order/$tok');
      expect(res.statusCode, 200);
      tokenHeaders(res, html, doc, '/order/$tok');
      expect(seen.where((r) => r.url.path == '/api/orders/$tok'), hasLength(1));
      final text = doc.body!.text;
      expect(doc.querySelector('h1')!.text, t.t('order.title', {'number': 'VX-100001'}));
      expect(text, contains(t.t('order.status.pending')));
      expect(text, contains('+88017****5678'));
      expect(text, contains('+88018****1111'));
      expect(text, contains('করিম'));
      expect(text, contains('Mirpur'));
      expect(text, contains('লাল চাল'));
      expect(text, contains('৫ কেজি'));
      expect(text, contains('৳১,৭৬০')); // total API er (Rule 1)
      expect(text, contains('৳৬০')); // delivery
      expect(text, contains(t.t('order.payment.cod')));
      final cancel = doc.querySelector('form[action="/_/orders/$tok/cancel"]')!;
      expect(cancel.attributes['method'], 'post');
      expect(cancel.querySelector('input[name="reason"]')!.attributes['maxlength'], '200');
      expect(cancel.querySelector('button[type="submit"]')!.text, t.t('order.cancel'));
    });

    test('English: /en/order e cancel form ?lang=en (no-referrer e Referer nai, 10.13 item 7)', () async {
      final (_, _, doc) = await page('/en/order/$tok');
      expect(doc.querySelector('form[action="/_/orders/$tok/cancel?lang=en"]'), isNotNull);
      expect(doc.body!.text, contains('Red rice'));
    });

    test('can_cancel false: cancel form na; cancelled e lekha', () async {
      canCancel = false;
      status = 'cancelled';
      final (_, _, doc) = await page('/order/$tok');
      expect(doc.querySelector('form[action^="/_/orders/"]'), isNull);
      expect(doc.body!.text, contains(t.t('order.cancelled')));
    });

    test('BFF er ?error=<code> lekha', () async {
      final (_, _, doc) = await page('/order/$tok?error=invalid_transition');
      expect(doc.querySelector('[role="alert"]')!.text, t.t('error.invalid_transition'));
    });

    test('token nai (404) ba bhul okkhor: 404 page, bhul okkhor e API call na', () async {
      final (res, _, _) = await page('/order/NOTATOKENNOTATOKENNOTATOKE');
      expect(res.statusCode, 404);
      final (bad, _, _) = await page('/order/a.b');
      expect(bad.statusCode, 404);
      expect(seen.where((r) => r.url.path.startsWith('/api/orders')), isEmpty);
    });
  });

  group('Done page /order/{token}/done (10.6)', () {
    test('number, pending e confirm SMS er kotha, confirm button na, account er kotha, order link', () async {
      final (res, html, doc) = await page('/order/$tok/done');
      expect(res.statusCode, 200);
      tokenHeaders(res, html, doc, '/order/$tok/done');
      final text = doc.body!.text;
      expect(doc.querySelector('h1')!.text, t.t('order.done.title'));
      expect(text, contains(t.t('order.done.number', {'number': 'VX-100001'})));
      expect(text, contains(t.t('order.done.confirm_sms')));
      expect(text, contains(t.t('order.done.signup')));
      expect(doc.querySelector('form'), isNull);
      expect(doc.querySelector('main a[href="/order/$tok"]')!.text, t.t('order.done.view'));
    });

    test('confirmed: confirm SMS er kotha na, shudhu SMS update', () async {
      status = 'confirmed';
      final (_, _, doc) = await page('/order/$tok/done');
      expect(doc.body!.text, isNot(contains(t.t('order.done.confirm_sms'))));
      expect(doc.body!.text, contains(t.t('order.done.sms')));
    });
  });

  group('COD confirm /confirm/{token} (8.10, 10.1)', () {
    test('pending: summary (mask) ar confirm form; token header', () async {
      final (res, html, doc) = await page('/confirm/$tok');
      expect(res.statusCode, 200);
      tokenHeaders(res, html, doc, '/confirm/$tok');
      expect(seen.where((r) => r.url.path == '/api/confirm/$tok'), hasLength(1));
      expect(doc.querySelector('h1')!.text, t.t('confirm.title'));
      expect(doc.body!.text, contains('+88017****5678'));
      expect(doc.body!.text, contains('VX-100001'));
      final form = doc.querySelector('form[action="/_/confirm/$tok"]')!;
      expect(form.attributes['method'], 'post');
      expect(form.querySelector('button[type="submit"]')!.text, t.t('confirm.submit'));
    });

    test('confirm hoye gele: done lekha, form na', () async {
      status = 'confirmed';
      final (_, _, doc) = await page('/confirm/$tok');
      expect(doc.body!.text, contains(t.t('confirm.done')));
      expect(doc.querySelector('form[action^="/_/confirm/"]'), isNull);
    });

    test('ochena token 404', () async {
      final (res, _, _) = await page('/confirm/NOTATOKENNOTATOKENNOTATOKE');
      expect(res.statusCode, 404);
    });
  });

  group('/account/orders (10.13 item 5)', () {
    test('login chhara 303 /login?next=, API call na', () async {
      final (res, _, _) = await page('/account/orders');
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/login?next=%2Faccount%2Forders');
      expect(seen, isEmpty);
    });

    test('login: Bearer diye /api/me/orders, card e order link, status, total; porer page link', () async {
      final (res, _, doc) = await page('/account/orders', cookie: '__Host-sid=GOOD');
      expect(res.statusCode, 200);
      expect(seen.singleWhere((r) => r.url.path == '/api/me/orders').headers['Authorization'], 'Bearer GOOD');
      expect(doc.querySelector('h1')!.text, t.t('account.orders'));
      expect(doc.querySelector('a[href="/order/TOKEN2TOKEN2TOKEN2TOKEN2TO"]')!.text, contains('VX-100002'));
      expect(doc.body!.text, contains(t.t('order.status.confirmed')));
      expect(doc.body!.text, contains('৳৫০০'));
      expect(doc.querySelector('a[href="/account/orders?page=2"]'), isNotNull);
      expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex');
    });

    test('?page=2 API te jay; session expire e 303', () async {
      await page('/account/orders?page=2', cookie: '__Host-sid=GOOD');
      expect(seen.singleWhere((r) => r.url.path == '/api/me/orders').url.queryParameters['page'], '2');
      final (res, _, _) = await page('/account/orders', cookie: '__Host-sid=OLD');
      expect(res.statusCode, 303);
    });

    test('/account e orders link', () async {
      final (_, _, doc) = await page('/account', cookie: '__Host-sid=GOOD');
      expect(doc.querySelector('main a[href="/account/orders"]')!.text, t.t('account.orders'));
    });
  });
}
