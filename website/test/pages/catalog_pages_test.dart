import 'package:html/dom.dart' as dom;
import 'package:jaspr/server.dart';
import 'package:jaspr_test/server_test.dart';
import 'package:website/main.server.options.dart';

import '../fake_site.dart';

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  group('Category page (10.1, 10.5)', () {
    testServer('naam, koyta, card, dam, srcset, filter, paging; query API te poisha', (t) async {
      requests.clear();
      t.pumpComponent(site({}));
      final res = await t.request('/c/lal?in_stock=1&min=100&sort=bad&page=1');
      expect(res.statusCode, 200);
      final doc = res.document!;
      expect(doc.querySelector('h1')!.text, 'লাল চাল');
      expect(doc.body!.text, contains('৩০টি পণ্য'));
      expect(doc.querySelectorAll('a[href="/p/red-rice"]'), hasLength(1));
      expect(doc.body!.text, contains('৳১৮০'));
      expect(doc.body!.text, contains('স্টকে নেই'));
      final img = doc.querySelector('img[alt="নাম red-rice"]')!;
      expect(img.attributes['srcset'], contains('width=1280,format=auto/shops/1/rice.jpg 1280w'));
      expect(img.attributes['fetchpriority'], 'high');
      expect(doc.querySelector('dialog#filters'), isNotNull);
      expect(doc.querySelector('a[rel="next"]')!.attributes['href'], '/c/lal?in_stock=1&min=100&page=2');
      final list = requests.singleWhere((u) => u.path == '/api/products');
      expect(list.queryParameters, {
        'category': 'lal',
        'in_stock': 'true',
        'min_price': '10000',
        'page': '1',
        'limit': '24',
      });
    });

    testServer('ochena category → 404', (t) async {
      t.pumpComponent(site({}));
      expect((await t.request('/c/nai')).statusCode, 404);
    });
  });

  group('Product card ar listing (10.5 v1.38, 10.7)', () {
    testServer('card: naam h2 (h1 er por, h3 na), discount badge (5% er kom na), stock shesh lekha', (t) async {
      for (final (locale, off, out) in [('bn', '১০% ছাড়', 'স্টকে নেই'), ('en', '10% off', 'Out of stock')]) {
        t.pumpComponent(site({}, locale: locale));
        final doc = (await t.request('/c/lal')).document!;
        final prefix = locale == 'en' ? '/en' : '';
        final red = doc.querySelector('a[href="$prefix/p/red-rice"]')!;
        final chini = doc.querySelector('a[href="$prefix/p/chinigura"]')!;
        expect(red.querySelector('h2')!.text, 'নাম red-rice', reason: locale);
        expect(doc.querySelectorAll('h3'), isEmpty, reason: locale);
        expect(red.text, contains(off), reason: locale);
        expect(chini.text, isNot(contains('%')), reason: locale); // 3%
        // Rong-er upor nirbhor na: lekha thake (10.7).
        expect(chini.text, contains(out), reason: locale);
        expect(red.text, isNot(contains(out)), reason: locale);
      }
    });

    testServer('khali list: icon, lekha, category te link', (t) async {
      t.pumpComponent(site({}));
      final doc = (await t.request('/search?q=nothing')).document!;
      final main = doc.querySelector('main')!;
      expect(main.text, contains('কিছু পাওয়া যায়নি'));
      expect(main.querySelector('svg[aria-hidden="true"]'), isNotNull);
      expect(main.querySelector('a[href="/#categories"]'), isNotNull);
      expect(main.querySelectorAll('a[href^="/p/"]'), isEmpty);
    });
  });

  group('Home (10.1)', () {
    testServer('category, featured (na pawa slug bad), banner er bhul link e link na', (t) async {
      t.pumpComponent(
        site({
          'homepage': {
            'banners': [
              {'image_key': 'shops/1/b1.jpg', 'link': '//evil.com'},
            ],
            'featured_product_slugs': ['nai'],
          },
        }),
      );
      final res = await t.request('/');
      expect(res.statusCode, 200);
      final doc = res.document!;
      expect(doc.querySelector('#categories a[href="/c/chal"]'), isNotNull);
      // Chobi chhara category tile e icon (10.5 v1.38 item 5).
      expect(doc.querySelector('#categories a[href="/c/chal"] svg[aria-hidden="true"]'), isNotNull);
      expect(doc.querySelector('a[href="//evil.com"]'), isNull);
      expect(doc.querySelector('img[srcset*="shops/1/b1.jpg"]'), isNotNull);
    });
    testServer('banner na thakle dekha hero: naam h1, tagline, category CTA (10.5 v1.38, plan 5)', (t) async {
      t.pumpComponent(
        site({
          'branding': {
            'name': {'bn': 'সবুজ বাজার'},
            'tagline': {'bn': 'কৃষকের কাছ থেকে সরাসরি'},
          },
        }),
      );
      final doc = (await t.request('/')).document!;
      final h1 = doc.querySelector('h1')!;
      expect(h1.text, 'সবুজ বাজার');
      expect(h1.classes, isNot(contains('sr-only')));
      final hero = h1.parent!;
      expect(hero.text, contains('কৃষকের কাছ থেকে সরাসরি'));
      expect(hero.querySelector('a[href="#categories"]')!.text, 'সব ক্যাটাগরি দেখুন');
    });

    testServer('banner thakle hero na: h1 sr-only, banner LCP', (t) async {
      t.pumpComponent(
        site({
          'branding': {
            'name': {'bn': 'সবুজ বাজার'},
            'tagline': {'bn': 'কৃষকের কাছ থেকে সরাসরি'},
          },
          'homepage': {
            'banners': [
              {'image_key': 'shops/1/b1.jpg'},
            ],
          },
        }),
      );
      final doc = (await t.request('/')).document!;
      expect(doc.querySelector('h1')!.classes, contains('sr-only'));
      expect(doc.body!.text, isNot(contains('কৃষকের কাছ থেকে সরাসরি')));
      expect(doc.querySelector('img[srcset*="shops/1/b1.jpg"]')!.attributes['fetchpriority'], 'high');
    });

    testServer('trust strip: shudhu settings e ja shotto (10.5 v1.38 item 3, 16.1 MU Q3)', (t) async {
      // Kichu na thakle strip-i na.
      t.pumpComponent(site({}));
      expect((await t.request('/')).document!.querySelector('#trust'), isNull);

      // COD + return; certification feature ache kintu shop er certificate settings e nai → na.
      // Delivery din: home e zone API call na, tai na.
      for (final (locale, cod, ret) in [
        ('bn', 'ক্যাশ অন ডেলিভারি', '৩ দিনের মধ্যে ফেরত'),
        ('en', 'Cash on delivery', 'Returns within 3 days'),
      ]) {
        t.pumpComponent(
          site({
            'features': ['certification'],
            'checkout': {
              'payment_methods': ['cod', 'sslcommerz'],
              'return_window_days': 3,
            },
          }, locale: locale),
        );
        final trust = (await t.request(locale == 'en' ? '/en' : '/')).document!.querySelector('#trust')!;
        final items = trust.querySelectorAll('li');
        expect(items, hasLength(2), reason: locale);
        expect(items[0].text, contains(cod), reason: locale);
        expect(items[1].text, contains(ret), reason: locale);
        expect(items.every((li) => li.querySelector('svg[aria-hidden="true"]') != null), isTrue, reason: locale);
      }

      // COD na (shudhu online), return na → strip na.
      t.pumpComponent(
        site({
          'checkout': {
            'payment_methods': ['sslcommerz'],
          },
        }),
      );
      expect((await t.request('/')).document!.querySelector('#trust'), isNull);
    });
  });

  group('Product page (10.1, 10.5)', () {
    testServer('gallery, variant, dam, rating, description escape, sticky bar; bottom nav na', (t) async {
      t.pumpComponent(site({}));
      final res = await t.request('/p/red-rice');
      expect(res.statusCode, 200);
      final doc = res.document!;
      expect(doc.querySelector('h1')!.text, 'লাল চাল');
      expect(doc.body!.text, contains('৪.৫ (২৪টি রিভিউ)'));
      // Gallery: scroll-snap, prothom ta LCP, baki lazy, alt = naam.
      final imgs = doc.querySelectorAll('.snap-x img');
      expect(imgs, hasLength(2));
      expect(imgs.first.attributes['fetchpriority'], 'high');
      expect(imgs.last.attributes['loading'], 'lazy');
      expect(imgs.every((i) => i.attributes['alt'] == 'লাল চাল' && i.attributes['width'] == '800'), isTrue);
      // Stock e thaka variant (8) shuru te bachai; dam API theke.
      // v1.28: radio er naam `variant_id` (cart e add er JS chhara form er field, 10.12 item 4).
      expect(doc.querySelector('input[name="variant_id"][value="8"]')!.attributes.containsKey('checked'), isTrue);
      expect(doc.querySelector('input[name="variant_id"][value="7"]')!.attributes.containsKey('checked'), isFalse);
      final shown = doc.querySelectorAll('[data-variant="8"]');
      expect(shown, hasLength(2)); // page ar sticky bar
      expect(shown.every((e) => !e.attributes.containsKey('hidden') && e.text.contains('৳৮৫০')), isTrue);
      expect(doc.querySelectorAll('[data-variant="7"]').every((e) => e.attributes.containsKey('hidden')), isTrue);
      expect(doc.querySelector('.fixed [data-variant="8"]')!.text, contains('৫ কেজি'));
      // Plain text: escape, khali line → <p>, newline → <br>.
      expect(doc.querySelector('script:not([src]):not([type="application/ld+json"])'), isNull);
      final paras = doc.querySelectorAll('section p');
      expect(paras.map((e) => e.text), ['প্রথম <script>x</script>দ্বিতীয়', 'তৃতীয়']);
      expect(paras.first.querySelectorAll('br'), hasLength(1));
      expect(doc.querySelector('nav.fixed'), isNull);
    });

    testServer('ochena product → 404', (t) async {
      t.pumpComponent(site({}));
      final res = await t.request('/p/nai');
      expect(res.statusCode, 404);
      expect('<main'.allMatches(res.body).length, 1); // Layout er main er bhetore main na (10.7)
    });
  });

  group('Search ar policy (10.1, 10.4)', () {
    testServer('search: q API te, noindex, filter e q thake; 2 okkhorer kom e API call na', (t) async {
      requests.clear();
      t.pumpComponent(site({}));
      final res = await t.request('/search?q=%20লাল%20&in_stock=1');
      expect(res.statusCode, 200);
      final doc = res.document!;
      expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex');
      expect(doc.querySelector('h1')!.text, '“লাল” এর ফলাফল');
      expect(doc.querySelector('a[href="/p/red-rice"]'), isNotNull);
      expect(doc.querySelector('form[action="/search"] input[type="hidden"][name="q"]')!.attributes['value'], 'লাল');
      expect(requests.singleWhere((u) => u.path == '/api/products').queryParameters, {
        'q': 'লাল',
        'in_stock': 'true',
        'page': '1',
        'limit': '24',
      });

      requests.clear();
      final short = await t.request('/search?q=ল');
      expect(short.statusCode, 200);
      expect(short.document!.body!.text, contains('কী খুঁজছেন লিখুন'));
      expect(requests.where((u) => u.path == '/api/products'), isEmpty);
    });

    testServer('policy: shironam, plain text escape; footer e link; ochena kind 404', (t) async {
      t.pumpComponent(site({}));
      final res = await t.request('/policy/terms');
      expect(res.statusCode, 200);
      final doc = res.document!;
      expect(doc.querySelector('h1')!.text, 'শর্তাবলি');
      expect(doc.querySelectorAll('article p').map((e) => e.text), ['প্রথম <b>লাইন</b>', 'দ্বিতীয়']);
      expect(doc.querySelector('article b'), isNull);
      for (final k in ['terms', 'return', 'refund', 'delivery', 'privacy']) {
        expect(doc.querySelector('footer a[href="/policy/$k"]'), isNotNull, reason: k);
      }
      expect((await t.request('/policy/faq')).statusCode, 404);
      expect((await t.request('/policy/privacy')).statusCode, 404); // fake API e nai → not_found
    });
  });

  group('Layout landmark (10.5, 10.7 v1.38)', () {
    testServer('protiti page e header/main/footer ekbar, ekta h1', (t) async {
      t.pumpComponent(site({}));
      for (final path in ['/', '/c/lal', '/p/red-rice', '/search?q=লাল', '/policy/terms', '/cart']) {
        final doc = (await t.request(path)).document!;
        for (final tag in ['header', 'main', 'footer', 'h1']) {
          expect(doc.querySelectorAll(tag), hasLength(1), reason: '$path $tag');
        }
      }
    });

    // (href, header er bhetore kina) protiti `aria-current="page"` link er.
    List<(String?, bool)> current(dom.Document doc) => [
      for (final a in doc.querySelectorAll('a[aria-current="page"]')) (a.attributes['href'], _inHeader(a)),
    ];

    testServer('aria-current="page": header ar bottom nav e ekhon er page er link', (t) async {
      t.pumpComponent(site({}));
      expect(current((await t.request('/cart')).document!), [('/cart', true), ('/cart', false)]);
      expect(current((await t.request('/')).document!), [('/', false)]);
      expect(current((await t.request('/c/lal')).document!), isEmpty);
    });

    testServer('logo na thakle naam + leaf icon; thakle chobi (alt = naam)', (t) async {
      final name = {
        'name': {'bn': 'সবুজ বাজার'},
      };
      t.pumpComponent(site({'branding': name}));
      var brand = (await t.request('/c/lal')).document!.querySelector('header a[href="/"]')!;
      expect(brand.text.trim(), 'সবুজ বাজার');
      expect(brand.querySelector('svg[aria-hidden="true"]'), isNotNull);
      expect(brand.querySelector('img'), isNull);

      t.pumpComponent(
        site({
          'branding': {...name, 'logo_key': 'shops/1/logo.png'},
        }),
      );
      brand = (await t.request('/c/lal')).document!.querySelector('header a[href="/"]')!;
      expect(brand.querySelector('img')!.attributes['alt'], 'সবুজ বাজার');
      expect(brand.querySelector('svg'), isNull);
    });

    testServer('footer: naam, phone tel: link, email, policy, DBID/BFSA', (t) async {
      t.pumpComponent(
        site({
          'branding': {
            'name': {'bn': 'সবুজ বাজার'},
            'phone': '+8801711000000',
            'email': 'hi@shop.test',
            'dbid_number': '123',
            'bfsa_number': '456',
          },
        }),
      );
      final footer = (await t.request('/')).document!.querySelector('footer')!;
      expect(footer.text, contains('সবুজ বাজার'));
      expect(footer.querySelector('a[href="tel:+8801711000000"]')!.text, contains('+8801711000000'));
      expect(footer.querySelector('a[href="mailto:hi@shop.test"]'), isNotNull);
      for (final k in ['terms', 'return', 'refund', 'delivery', 'privacy']) {
        expect(footer.querySelector('nav a[href="/policy/$k"]'), isNotNull, reason: k);
      }
      expect(footer.text, contains('DBID: 123'));
      expect(footer.text, contains('BFSA নিবন্ধন: 456'));
    });
  });
}

bool _inHeader(dom.Element e) {
  for (var p = e.parent; p != null; p = p.parent) {
    if (p.localName == 'header') return true;
  }
  return false;
}
