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
      expect(doc.querySelector('a[href="//evil.com"]'), isNull);
      expect(doc.querySelector('img[srcset*="shops/1/b1.jpg"]'), isNotNull);
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
      expect(doc.querySelector('input[name="variant"][value="8"]')!.attributes.containsKey('checked'), isTrue);
      expect(doc.querySelector('input[name="variant"][value="7"]')!.attributes.containsKey('checked'), isFalse);
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
      expect(res.document!.querySelectorAll('main'), hasLength(1)); // Layout er main er bhetore main na (10.7)
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
}
