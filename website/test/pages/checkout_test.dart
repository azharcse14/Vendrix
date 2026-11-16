import 'dart:convert';

import 'package:html/dom.dart' as dom;
import 'package:html/parser.dart' show parse;
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';
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
    'TURNSTILE_SITE_KEY': '1x00000000000000000000AA',
  });
  const t = Translator('bn');
  const key = 'AAAABBBBCCCCDDDDEEEEFFFF22';
  const json = {'content-type': 'application/json; charset=utf-8'};

  final cart = {
    'items': [
      {
        'variant_id': 8,
        'product_slug': 'red-rice',
        'name': 'লাল চাল',
        'variant_label': '৫ কেজি',
        'image': null,
        'unit_price': 85000,
        'compare_at_price': null,
        'variable_weight': false,
        'quantity': 2,
        'max_quantity': 10,
        'in_stock': true,
        'line_total': 170000,
      },
    ],
    'count': 1,
    'subtotal': 170000,
    'notices': <Object>[],
  };
  final zones = [
    {
      'district': 'Dhaka',
      'areas': [
        {'area': 'Mirpur', 'name': 'মিরপুর'},
        {'area': 'Dhanmondi', 'name': 'ধানমন্ডি'},
      ],
      'district_wide': true,
    },
    {
      'district': 'Chattogram',
      'areas': [
        {'area': 'Agrabad', 'name': 'আগ্রাবাদ'},
      ],
      'district_wide': false,
    },
  ];

  late List<http.Request> seen;
  var emptyCart = false;
  final api = MockClient((r) async {
    seen.add(r);
    final auth = r.headers['Authorization'];
    if (auth != null && auth != 'Bearer GOOD') {
      return http.Response(
        jsonEncode({
          'error': {'code': 'session_expired', 'message': ''},
        }),
        401,
        headers: json,
      );
    }
    final body = switch (r.url.path) {
      '/api/cart' => emptyCart ? {...cart, 'items': <Object>[], 'count': 0, 'subtotal': 0} : cart,
      '/api/cart/count' => {'count': emptyCart ? 0 : 1},
      '/api/delivery-zones' => zones,
      '/api/me' => {'phone': '01712345678', 'name': 'রহিম', 'email': 'r@example.com', 'locale': 'bn'},
      _ => null,
    };
    if (body == null) return http.Response('{"error":{"code":"not_found","message":""}}', 404, headers: json);
    return http.Response(jsonEncode(body), 200, headers: json);
  });

  Future<ShopSettings> shop(String _) async => const ShopSettings(
    PublicSettings({
      'checkout': {
        'payment_methods': ['cod'],
      },
    }),
    1,
    {},
  );

  Future<(Response, String, dom.Document)> page(String path, {String? cookie}) async {
    seen = [];
    final handler = serveApp((r, render) => renderPage(r, render, shop, cfg: cfg, client: api, log: (_) {}));
    final res = await handler(
      Request('GET', Uri.parse('http://shop.test$path'), headers: {'host': 'shop.test', 'cookie': ?cookie}),
    );
    final html = await res.readAsString();
    return (res, html, parse(html));
  }

  setUp(() => emptyCart = false);

  group('Checkout page (10.1, 10.6, 10.13 item 1–3)', () {
    test('guest: cart ar zone SSR e, form er field (10.5 item 5), Turnstile, @client marker, noindex', () async {
      final (res, html, doc) = await page('/checkout', cookie: '__Host-cart=$key');
      expect(res.statusCode, 200);
      expect(seen.singleWhere((r) => r.url.path == '/api/cart').headers['X-Cart-Key'], key);
      expect(seen.where((r) => r.url.path == '/api/delivery-zones'), hasLength(1));
      expect(seen.where((r) => r.url.path == '/api/me'), isEmpty);
      expect(doc.querySelector('h1')!.text, t.t('checkout.title'));
      expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex');

      final form = doc.querySelector('form#checkout')!;
      expect(form.attributes['method'], 'post');
      expect(form.attributes['action'], '/_/checkout');
      dom.Element field(String name) => form.querySelector('[name="$name"]')!;
      expect(field('customer.name').attributes['autocomplete'], 'name');
      expect(field('customer.name').attributes['maxlength'], '100');
      final phone = field('customer.phone');
      expect(phone.attributes['type'], 'tel');
      expect(phone.attributes['inputmode'], 'numeric');
      expect(phone.attributes['autocomplete'], 'tel');
      expect(phone.attributes.containsKey('readonly'), isFalse);
      expect(phone.classes, contains('text-base'));
      expect(field('customer.email').attributes['type'], 'email');
      expect(field('address.line').attributes['autocomplete'], 'street-address');
      expect(field('address.line').attributes['maxlength'], '500');
      expect(field('note').attributes['maxlength'], '500');
      expect(field('coupon_code').attributes['maxlength'], '32');
      // Protiti input label er bhetore (10.7).
      for (final n in ['customer.name', 'customer.phone', 'address.line', 'coupon_code']) {
        expect(field(n).parent!.localName, 'label', reason: n);
      }

      // District ar protiti district er area select (8.19 item 2); "Onno area" shudhu district-wide e.
      final district = field('address.district');
      expect([for (final o in district.querySelectorAll('option')) o.attributes['value']], ['Dhaka', 'Chattogram']);
      final dhaka = form.querySelector('select[data-district="Dhaka"]')!;
      expect([for (final o in dhaka.querySelectorAll('option')) o.attributes['value']], ['Mirpur', 'Dhanmondi', '']);
      expect(dhaka.querySelector('option[value=""]')!.text, t.t('checkout.area_other'));
      expect(dhaka.querySelector('option[value="Mirpur"]')!.text, 'মিরপুর');
      final ctg = form.querySelector('select[data-district="Chattogram"]')!;
      expect([for (final o in ctg.querySelectorAll('option')) o.attributes['value']], ['Agrabad']);
      // Prothom district chhara baki select lukano ar disabled.
      expect(dhaka.attributes.containsKey('hidden'), isFalse);
      expect(ctg.attributes.containsKey('hidden'), isTrue);
      expect(ctg.attributes.containsKey('disabled'), isTrue);
      expect(field('area_other').attributes['maxlength'], '100');

      // Recipient (off thakle lukano), payment (settings theke), consent, honeypot.
      expect(form.querySelector('input[name="other_recipient"]')!.attributes['type'], 'checkbox');
      expect(form.querySelector('#checkout-recipient')!.attributes.containsKey('hidden'), isTrue);
      expect(field('recipient.phone').attributes['type'], 'tel');
      final pay = form.querySelectorAll('input[name="payment_method"]');
      expect([for (final p in pay) p.attributes['value']], ['cod']);
      expect(pay.single.attributes.containsKey('checked'), isTrue);
      expect(field('consent').attributes['type'], 'checkbox');
      expect(field('consent').attributes.containsKey('required'), isTrue);
      final honeypot = field('website');
      expect(honeypot.attributes['tabindex'], '-1');
      expect(honeypot.attributes['autocomplete'], 'off');

      // OTP box (10.6): lukano, one-time-code.
      final otp = form.querySelector('#checkout-otp')!;
      expect(otp.attributes.containsKey('hidden'), isTrue);
      expect(otp.querySelector('input[name="otp_code"]')!.attributes['autocomplete'], 'one-time-code');

      final widget = form.querySelector('.cf-turnstile')!;
      expect(widget.attributes['data-sitekey'], '1x00000000000000000000AA');
      expect(widget.attributes['data-response-field-name'], 'turnstile_token');
      expect(
        doc.querySelector('script[src="https://challenges.cloudflare.com/turnstile/v0/api.js"]'),
        isNotNull,
      );

      // Cart er line, total quote theke (Rule 1): shuru te faka, submit disabled.
      expect(doc.body!.text, contains('লাল চাল'));
      expect(doc.querySelector('[data-quote="total"]')!.text, isEmpty);
      expect(form.querySelector('button[type="submit"]')!.attributes.containsKey('disabled'), isTrue);
      // Bhul er lekha server e (client shudhu dekhay).
      expect(doc.querySelector('[data-error="price_changed"]')!.text, t.t('error.price_changed'));
      expect(doc.querySelector('[data-error="area_not_served"]')!.text, t.t('error.area_not_served'));
      expect(doc.querySelector('#checkout-error')!.attributes['role'], 'alert');
      // OTP er lekha `{phone}` client bosay.
      expect(doc.querySelector('[data-otp-text]')!.text, t.t('checkout.otp_sent', {'phone': '{phone}'}));
      expect(html, contains('<!--@checkout_form data={"digits":"০১২৩৪৫৬৭৮৯","orderBase":"/order/"}-->'));
    });

    test('login: /api/me Bearer diye; phone readonly (8.19 item 4), naam/email prefill', () async {
      final (res, _, doc) = await page('/checkout', cookie: '__Host-sid=GOOD');
      expect(res.statusCode, 200);
      expect(seen.singleWhere((r) => r.url.path == '/api/me').headers['Authorization'], 'Bearer GOOD');
      final phone = doc.querySelector('[name="customer.phone"]')!;
      expect(phone.attributes['value'], '01712345678');
      expect(phone.attributes.containsKey('readonly'), isTrue);
      expect(doc.querySelector('[name="customer.name"]')!.attributes['value'], 'রহিম');
      expect(doc.querySelector('[name="customer.email"]')!.attributes['value'], 'r@example.com');
    });

    test('session expire (401): 303 /login?next=/checkout', () async {
      final (res, _, _) = await page('/checkout', cookie: '__Host-sid=OLD');
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/login?next=%2Fcheckout');
    });

    test('cookie nai ba cart khali: form na, cart er link; zone call na', () async {
      for (final cookie in [null, '__Host-cart=$key']) {
        emptyCart = true;
        final (res, html, doc) = await page('/checkout', cookie: cookie);
        expect(res.statusCode, 200, reason: cookie);
        expect(doc.querySelector('form#checkout'), isNull, reason: cookie);
        expect(doc.body!.text, contains(t.t('cart.empty')), reason: cookie);
        expect(doc.querySelector('main a[href="/cart"]'), isNotNull, reason: cookie);
        expect(seen.where((r) => r.url.path == '/api/delivery-zones'), isEmpty, reason: cookie);
        expect(html, isNot(contains('<!--@checkout_form')), reason: cookie);
      }
    });

    test('/cart e item thakle checkout link (10.12 item 4 er "M6 porjonto" shesh)', () async {
      final (_, _, doc) = await page('/cart', cookie: '__Host-cart=$key');
      expect(doc.querySelector('a[href="/checkout"]')!.text, t.t('cart.checkout'));
      emptyCart = true;
      final (_, _, empty) = await page('/cart', cookie: '__Host-cart=$key');
      expect(empty.querySelector('a[href="/checkout"]'), isNull);
    });
  });
}
