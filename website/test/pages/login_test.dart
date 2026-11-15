import 'dart:convert';

import 'package:html/parser.dart' show parse;
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';
import 'package:test/test.dart';
import 'package:website/main.server.options.dart';
import 'package:website/pages/login.dart';
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
  late List<http.Request> seen;
  // Fake Go API: `/api/me` shudhu thik token e (7.3), na hole 401.
  final api = MockClient((r) async {
    seen.add(r);
    if (r.url.path == '/api/me' && r.headers['Authorization'] == 'Bearer GOOD') {
      return http.Response(
        jsonEncode({'phone': '01712345678', 'name': 'রহিম', 'email': null, 'locale': 'en'}),
        200,
        headers: {'content-type': 'application/json; charset=utf-8'},
      );
    }
    return http.Response(
      jsonEncode({
        'error': {'code': 'session_expired', 'message': ''},
      }),
      401,
    );
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

  Future<Response> get(String path, {String? cookie}) async {
    seen = [];
    final handler = serveApp((r, render) => renderPage(r, render, shop, cfg: cfg, client: api, log: (_) {}));
    return handler(
      Request(
        'GET',
        Uri.parse('http://shop.test$path'),
        headers: {'host': 'shop.test', 'cookie': ?cookie},
      ),
    );
  }

  group('Login page (10.1)', () {
    test('phone form: tel input (16px), Turnstile widget ar script, code form lukano, noindex', () async {
      final res = await get('/login');
      expect(res.statusCode, 200);
      final html = await res.readAsString();
      final doc = parse(html);
      final phone = doc.querySelector('#login-phone input[name="phone"]')!;
      expect(phone.attributes['type'], 'tel');
      expect(phone.attributes['inputmode'], 'numeric');
      expect(phone.attributes['autocomplete'], 'tel');
      expect(phone.classes, contains('text-base'));
      expect(doc.querySelector('#login-phone input[name="purpose"]')!.attributes['value'], 'login');
      final widget = doc.querySelector('#login-phone .cf-turnstile')!;
      expect(widget.attributes['data-sitekey'], '1x00000000000000000000AA');
      expect(widget.attributes['data-response-field-name'], 'turnstile_token');
      expect(widget.attributes['data-language'], 'bn');
      expect(
        doc.querySelector('script[src="https://challenges.cloudflare.com/turnstile/v0/api.js"]'),
        isNotNull,
      );
      final code = doc.querySelector('#login-code')!;
      expect(code.attributes.containsKey('hidden'), isTrue);
      final input = code.querySelector('input[name="code"]')!;
      expect(input.attributes['inputmode'], 'numeric');
      expect(input.attributes['autocomplete'], 'one-time-code');
      expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex');
      // Bhul code er lekha server e (client e shop er lekha jay na).
      expect(doc.querySelector('[data-error="otp_invalid"]')!.text, 'কোডটি ঠিক নেই বা মেয়াদ শেষ');
      // @client marker e shudhu next.
      expect(html, contains('<!--@login_form data={"next":"/account"}-->'));
      expect(seen, isEmpty);
    });

    test('English: /en/login lekha, next default /en/account', () async {
      final html = await (await get('/en/login')).readAsString();
      expect(parse(html).querySelector('h1')!.text, 'Log in');
      expect(html, contains('data={"next":"/en/account"}'));
    });

    test('nijer path er next thake; onno host er next default e', () async {
      expect(await (await get('/login?next=%2Fcart')).readAsString(), contains('data={"next":"/cart"}'));
      expect(await (await get('/login?next=%2F%2Fevil.com')).readAsString(), contains('data={"next":"/account"}'));
    });

    test('safeNext: shudhu "/" diye shuru, "//", "\\" ba control okkhor na (10.11 item 1)', () {
      for (final ok in ['/', '/account', '/p/লাল-চাল?x=1']) {
        expect(safeNext(ok, '/account'), ok, reason: ok);
      }
      for (final bad in [
        null,
        '',
        'account',
        '//evil.com',
        r'/\evil.com',
        '/\t/evil.com',
        'https://evil.com',
        '/a\nb',
      ]) {
        expect(safeNext(bad, '/account'), '/account', reason: '$bad');
      }
    });
  });

  group('Account page (10.1, 10.11 item 5)', () {
    test('cookie nai → 303 /login?next=, API call na', () async {
      final res = await get('/account');
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/login?next=%2Faccount');
      expect(seen, isEmpty);
      final en = await get('/en/account');
      expect(en.headers['location'], '/en/login?next=%2Fen%2Faccount');
    });

    test('session expire (API 401) → 303 /login', () async {
      final res = await get('/account', cookie: '__Host-sid=OLD');
      expect(res.statusCode, 303);
      expect(res.headers['location'], '/login?next=%2Faccount');
      expect(seen.single.headers['Authorization'], 'Bearer OLD');
    });

    test('session thakle /api/me Bearer diye: naam, phone, bhasha form, logout form', () async {
      final res = await get('/account', cookie: '__Host-sid=GOOD');
      expect(res.statusCode, 200);
      expect(seen.where((r) => r.url.path == '/api/me').single.headers['Authorization'], 'Bearer GOOD');
      final doc = parse(await res.readAsString());
      expect(doc.body!.text, contains('01712345678'));
      final form = doc.querySelector('form[action="/_/account"]')!;
      expect(form.attributes['method'], 'post');
      final name = form.querySelector('input[name="name"]')!;
      expect(name.attributes['value'], 'রহিম');
      expect(name.attributes['autocomplete'], 'name');
      expect(
        form.querySelector('select[name="locale"] option[value="en"]')!.attributes.containsKey('selected'),
        isTrue,
      );
      expect(doc.querySelector('form[action="/_/auth/logout"]')!.attributes['method'], 'post');
      expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex');
    });

    test('BFF er ?error=<code> lekha hishebe', () async {
      final doc = parse(
        await (await get('/account?error=validation_failed', cookie: '__Host-sid=GOOD')).readAsString(),
      );
      expect(doc.querySelector('[role="alert"]')!.text, 'তথ্যে ভুল আছে');
    });
  });

  group('Account link (10.10 item 6, v1.19)', () {
    test('header ar bottom nav e /account', () async {
      final doc = parse(await (await get('/policy/nai')).readAsString());
      expect(doc.querySelectorAll('header a[href="/account"]'), hasLength(1));
      expect(doc.querySelectorAll('nav.fixed a[href="/account"]'), hasLength(1));
    });
  });
}
