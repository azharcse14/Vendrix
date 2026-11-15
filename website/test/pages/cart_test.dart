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

import '../fake_site.dart' show card, fakeApi;

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
  // Go er `rand.Text()` er format (8.4).
  const key = 'AAAABBBBCCCCDDDDEEEEFFFF22';
  const json = {'content-type': 'application/json; charset=utf-8'};

  Map<String, dynamic> item(
    int id,
    String slug,
    String name,
    String label,
    int price,
    int qty, {
    bool weight = false,
    bool inStock = true,
  }) => {
    'variant_id': id,
    'product_slug': slug,
    'name': name,
    'variant_label': label,
    'image': null,
    'unit_price': price,
    'compare_at_price': null,
    'variable_weight': weight,
    'quantity': qty,
    'max_quantity': 10,
    'in_stock': inStock,
    'line_total': inStock ? price * qty : 0,
  };
  final cart = {
    'items': [
      item(8, 'red-rice', 'লাল চাল', '৫ কেজি', 85000, 2),
      item(9, 'rui', 'রুই মাছ', '', 50000, 1, weight: true),
      item(7, 'red-rice', 'লাল চাল', '১ কেজি', 18000, 1, inStock: false),
    ],
    'count': 3,
    'subtotal': 220000,
    'notices': [
      {'code': 'item_removed', 'name': 'মধু'},
    ],
  };

  late List<http.Request> seen;
  // Fake Go API: cart/wishlist ekhane, catalog `fake_site` er; Bearer shudhu GOOD (7.3).
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
    switch (r.url.path) {
      case '/api/cart':
        return http.Response(jsonEncode(cart), 200, headers: json);
      case '/api/cart/count':
        return http.Response(jsonEncode({'count': 3}), 200, headers: json);
      case '/api/wishlist':
        return http.Response(
          jsonEncode({
            'items': [
              {...card('red-rice', 18000), 'id': 1},
            ],
          }),
          200,
          headers: json,
        );
    }
    return fakeApi.get(r.url); // r already finalize, tai notun request
  });

  Future<ShopSettings> shop(String _) async => const ShopSettings(PublicSettings({}), 1, {});

  Future<Response> get(String path, {String? cookie}) async {
    seen = [];
    final handler = serveApp((r, render) => renderPage(r, render, shop, cfg: cfg, client: api, log: (_) {}));
    return handler(
      Request('GET', Uri.parse('http://shop.test$path'), headers: {'host': 'shop.test', 'cookie': ?cookie}),
    );
  }

  Future<(Response, String, dom.Document)> page(String path, {String? cookie}) async {
    final res = await get(path, cookie: cookie);
    final html = await res.readAsString();
    return (res, html, parse(html));
  }

  Iterable<http.Request> cartCalls() => seen.where((r) => r.url.path == '/api/cart');
  Iterable<http.Request> countCalls() => seen.where((r) => r.url.path == '/api/cart/count');
  const sidCleared = '__Host-sid=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0';

  group('Cart page (10.1, 10.5, 10.12 item 4, 6)', () {
    test('guest __Host-cart: X-Cart-Key diye GET, item, qty ar delete form (JS chhara), notice, subtotal', () async {
      final (res, _, doc) = await page('/cart', cookie: '__Host-cart=$key');
      expect(res.statusCode, 200);
      final call = cartCalls().single;
      expect(call.headers['X-Cart-Key'], key);
      expect(call.headers.containsKey('Authorization'), isFalse);
      expect(doc.querySelector('h1')!.text, t.t('cart.title'));
      expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex');
      expect(doc.querySelector('a[href="/p/red-rice"]'), isNotNull);

      // Qty: JS chhara POST → BFF PATCH (10.9); shima API er max_quantity.
      final qty = doc.querySelector('form[action="/_/cart/items/8"]')!;
      expect(qty.attributes['method'], 'post');
      final input = qty.querySelector('input[name="quantity"]')!;
      expect(input.attributes['type'], 'number');
      expect(input.attributes['value'], '2');
      expect(input.attributes['min'], '1');
      expect(input.attributes['max'], '10');
      expect(input.classes, contains('text-base'));
      expect(qty.querySelector('button[type="submit"]')!.text, t.t('cart.update'));
      expect(input.parent!.localName, 'label'); // 10.7
      final del = doc.querySelector('form[action="/_/cart/items/8/delete"]')!;
      expect(del.attributes['method'], 'post');
      expect(del.querySelector('button[type="submit"]')!.text, t.t('cart.remove'));

      final text = doc.body!.text;
      expect(text, contains(t.t('cart.notice.item_removed', {'name': 'মধু'})));
      expect(text, contains('৳১,৭০০')); // line total API er
      expect(text, contains('৳২,২০০')); // subtotal API er (Rule 1)
      expect(text, contains(t.t('cart.approx'))); // ojon-variable line
      // Bracket l10n er bhetore (bhasha onujayi), code e na (v1.31).
      expect(t.t('cart.approx'), matches(RegExp(r'^\(.+\)$')));
      expect(text, isNot(contains('((')));
      expect(text, contains(t.t('product.out_of_stock')));
      // Checkout M6 e (10.12 item 4).
      expect(doc.querySelector('a[href*="checkout"], form[action*="checkout"]'), isNull);
    });

    test('login (__Host-sid): Bearer, X-Cart-Key na', () async {
      final (_, _, doc) = await page('/cart', cookie: '__Host-sid=GOOD; __Host-cart=$key');
      final call = cartCalls().single;
      expect(call.headers['Authorization'], 'Bearer GOOD');
      expect(call.headers.containsKey('X-Cart-Key'), isFalse);
      expect(doc.querySelector('form[action="/_/cart/items/9/delete"]'), isNotNull);
    });

    // `/en` er bhasha prefix BFF test e (ei fake shop e English nai).
    test('session expire (401): 303 /login?next=/cart, __Host-sid Max-Age=0 (10.12 item 7)', () async {
      final res = await get('/cart', cookie: '__Host-sid=OLD');
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/login?next=%2Fcart');
      expect(res.headersAll['set-cookie'], [sidCleared]);
    });

    test('bhul okkhor er __Host-sid: Bearer na, guest key diye (10.12 item 8)', () async {
      await page('/cart', cookie: '__Host-sid=a"b; __Host-cart=$key');
      final call = cartCalls().single;
      expect(call.headers.containsKey('Authorization'), isFalse);
      expect(call.headers['X-Cart-Key'], key);
    });

    test('cookie nai ba bhul key: API call na, khali cart', () async {
      for (final cookie in [null, '__Host-cart=bad']) {
        final (res, _, doc) = await page('/cart', cookie: cookie);
        expect(res.statusCode, 200);
        expect(cartCalls(), isEmpty, reason: cookie);
        expect(doc.body!.text, contains(t.t('cart.empty')));
        expect(doc.querySelector('form[action^="/_/cart/items/"]'), isNull);
      }
    });

    test('wl cookie e koma (10.12 item 3) thakleo cookie pora bhange na (dart:io Cookie koma mane na)', () async {
      final (res, _, _) = await page('/cart', cookie: 'wl=1,2,3; __Host-cart=$key');
      expect(res.statusCode, 200);
      expect(cartCalls().single.headers['X-Cart-Key'], key);
      final account = await get('/account/wishlist', cookie: 'wl=1,2; __Host-sid=GOOD');
      expect(account.statusCode, 200);
    });

    test('BFF er ?error=<code> lekha hishebe', () async {
      final (_, _, doc) = await page('/cart?error=out_of_stock', cookie: '__Host-cart=$key');
      expect(doc.querySelector('[role="alert"]')!.text, t.t('error.out_of_stock'));
    });
  });

  group('Wishlist page (/account/wishlist, 10.1, 10.12 item 3)', () {
    test('login chhara 303 /login?next=, API call na', () async {
      final res = await get('/account/wishlist');
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/login?next=%2Faccount%2Fwishlist');
      expect(seen, isEmpty);
      final expired = await get('/account/wishlist', cookie: '__Host-sid=OLD');
      expect(expired.statusCode, 303);
      expect(expired.headersAll['set-cookie'], [sidCleared]); // 10.12 item 7
      final account = await get('/account', cookie: '__Host-sid=OLD');
      expect(account.statusCode, 303);
      expect(account.headersAll['set-cookie'], [sidCleared]);
    });

    test('login e GET /api/wishlist Bearer diye: card ar bad dewar button', () async {
      final (res, html, doc) = await page('/account/wishlist', cookie: '__Host-sid=GOOD');
      expect(res.statusCode, 200);
      expect(seen.singleWhere((r) => r.url.path == '/api/wishlist').headers['Authorization'], 'Bearer GOOD');
      expect(doc.querySelector('h1')!.text, t.t('wishlist.title'));
      expect(doc.querySelector('a[href="/p/red-rice"]'), isNotNull);
      expect(doc.querySelector('button[data-wishlist="1"]')!.attributes['aria-pressed'], 'true');
      expect(html, contains('<!--@wishlist_button'));
    });
  });

  group('Cart link ar badge (10.10 item 6, 10.12 item 5)', () {
    test('cookie chhara: header ar bottom nav e /cart, badge khali, /api/cart call na (cache-safe)', () async {
      final (_, _, doc) = await page('/policy/nai');
      expect(doc.querySelectorAll('header a[href="/cart"]'), hasLength(1));
      expect(doc.querySelectorAll('nav.fixed a[href="/cart"]'), hasLength(1));
      for (final b in doc.querySelectorAll('[data-cart-badge]')) {
        expect(b.attributes.containsKey('hidden'), isTrue);
        expect(b.text, isEmpty);
      }
      expect(cartCalls(), isEmpty);
      expect(countCalls(), isEmpty);
    });

    test('__Host-cart thakle GET /api/cart/count ekbar, GET /api/cart na (notice gile na, v1.31)', () async {
      final (_, _, doc) = await page('/policy/nai', cookie: '__Host-cart=$key');
      expect(countCalls().single.headers['X-Cart-Key'], key);
      expect(cartCalls(), isEmpty);
      final badges = doc.querySelectorAll('[data-cart-badge]');
      expect(badges, isNotEmpty);
      expect(badges.every((b) => b.text == '৩' && !b.attributes.containsKey('hidden')), isTrue);
    });
  });

  group('Cart badge er session expire (10.12 item 5, 7)', () {
    test('401 e badge lukano, page bhange na (redirect na)', () async {
      final (res, _, doc) = await page('/policy/nai', cookie: '__Host-sid=OLD');
      expect(res.statusCode, isNot(303));
      expect(countCalls(), hasLength(1));
      for (final b in doc.querySelectorAll('[data-cart-badge]')) {
        expect(b.attributes.containsKey('hidden'), isTrue);
      }
    });

    test('/cart page e badge count theke, list GET /api/cart theke (duita alada call)', () async {
      await page('/cart', cookie: '__Host-cart=$key');
      expect(countCalls(), hasLength(1));
      expect(cartCalls(), hasLength(1));
    });
  });

  group('Product page: cart e add ar wishlist (10.5, 10.12 item 3, 4)', () {
    test('JS chhara form POST /_/cart/items: variant_id radio, quantity, sticky bar e submit', () async {
      final (_, html, doc) = await page('/p/red-rice');
      final form = doc.querySelector('form[action="/_/cart/items"]')!;
      expect(form.attributes['method'], 'post');
      expect(form.querySelector('input[name="variant_id"][value="8"]')!.attributes.containsKey('checked'), isTrue);
      final qty = form.querySelector('input[name="quantity"]')!;
      expect(qty.attributes['type'], 'number');
      expect(qty.attributes['value'], '1');
      expect(qty.attributes['min'], '1');
      expect(qty.classes, contains('text-base'));
      expect(form.querySelector('.fixed button[type="submit"]')!.text, t.t('cart.add'));
      expect(html, contains('<!--@add_to_cart'));
      // Guest er heart: shuru te off, cookie `wl` JS e (server kichu render kore na, page public).
      expect(doc.querySelector('button[data-wishlist="1"]')!.attributes['aria-pressed'], 'false');
      expect(html, contains('<!--@wishlist_button'));
      expect(seen.where((r) => r.url.path == '/api/cart' || r.url.path == '/api/wishlist'), isEmpty);
    });

    test('login e heart er shuru obostha GET /api/wishlist theke', () async {
      final (_, _, doc) = await page('/p/red-rice', cookie: '__Host-sid=GOOD');
      expect(doc.querySelector('button[data-wishlist="1"]')!.attributes['aria-pressed'], 'true');
    });

    test('JS chhara add er error (?error=out_of_stock) lekha', () async {
      final (_, _, doc) = await page('/p/red-rice?error=out_of_stock');
      expect(doc.querySelector('[role="alert"]')!.text, t.t('error.out_of_stock'));
    });
  });

  group('Login form er cart_notices (10.12 item 2, v1.30)', () {
    test('notice er lekha server e ({name} client bosay), box lukano', () async {
      final (_, _, doc) = await page('/login');
      expect(
        doc.querySelector('[data-notice="item_dropped"]')!.text,
        t.t('cart.notice.item_dropped', {'name': '{name}'}),
      );
      final box = doc.querySelector('#login-notice')!;
      expect(box.attributes['role'], 'status');
      expect(box.attributes.containsKey('hidden'), isTrue);
    });
  });
}
