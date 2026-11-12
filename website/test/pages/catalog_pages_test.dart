import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:jaspr_test/server_test.dart';
import 'package:shared/shared.dart';
import 'package:website/app.dart';
import 'package:website/components/site.dart';
import 'package:website/main.server.options.dart';
import 'package:website/services/image.dart';

// Fake Go API (7.2 er shape).
Map<String, dynamic> card(String slug, int price, {bool inStock = true, String? image}) => {
  'id': slug.length,
  'slug': slug,
  'name': 'নাম $slug',
  'unit': 'kg',
  'variable_weight': false,
  'price': price,
  'compare_at_price': null,
  'in_stock': inStock,
  'rating': 0,
  'review_count': 0,
  'image': image,
};

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  const json = {'content-type': 'application/json; charset=utf-8'};
  final requests = <Uri>[];
  final api = MockClient((r) async {
    requests.add(r.url);
    Object? body;
    switch (r.url.path) {
      case '/api/categories':
        body = [
          {
            'id': 1,
            'slug': 'chal',
            'name': 'চাল',
            'image_key': null,
            'children': [
              {'id': 2, 'slug': 'lal', 'name': 'লাল চাল', 'image_key': null, 'children': []},
            ],
          },
        ];
      case '/api/products':
        body = {
          'items': [card('red-rice', 18000, image: 'shops/1/rice.jpg'), card('chinigura', 16000, inStock: false)],
          'page': 1,
          'limit': 24,
          'total': 30,
        };
      case '/api/policies/terms':
        body = {'kind': 'terms', 'body': 'প্রথম <b>লাইন</b>\n\nদ্বিতীয়'};
      case '/api/products/red-rice':
        body = {
          'id': 1,
          'slug': 'red-rice',
          'name': 'লাল চাল',
          'description': 'প্রথম <script>x</script>\nদ্বিতীয়\n\nতৃতীয়',
          'unit': 'kg',
          'variable_weight': false,
          'category': 'lal',
          'attributes': <String, dynamic>{},
          'seo': <String, dynamic>{},
          'rating': 4.5,
          'review_count': 24,
          'variants': [
            for (final (id, w, price, stock) in [(7, '১ কেজি', 18000, false), (8, '৫ কেজি', 85000, true)])
              {
                'id': id,
                'sku': 'R$id',
                'options': {'weight': w},
                'price': price,
                'compare_at_price': null,
                'nominal_weight_g': null,
                'in_stock': stock,
              },
          ],
          'images': [
            for (final k in ['a', 'b']) {'storage_key': 'shops/1/$k.jpg', 'alt': '', 'width': 800, 'height': 600},
          ],
        };
      default:
        return http.Response(
          jsonEncode({
            'error': {'code': 'not_found', 'message': 'x'},
          }),
          404,
          headers: json,
        );
    }
    return http.Response(jsonEncode(body), 200, headers: json);
  });

  Component site(Map<String, dynamic> settings) => Site(
    locale: 'bn',
    settings: PublicSettings(settings),
    translator: const Translator('bn'),
    catalog: CatalogRepository(ApiClient(Uri.parse('http://api.test'), api)),
    images: ImageUrls(Uri.parse('https://img.test'), resize: true),
    child: const App(),
  );

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
      expect(doc.querySelector('script:not([src])'), isNull);
      final paras = doc.querySelectorAll('section p');
      expect(paras.map((e) => e.text), ['প্রথম <script>x</script>দ্বিতীয়', 'তৃতীয়']);
      expect(paras.first.querySelectorAll('br'), hasLength(1));
      expect(doc.querySelector('nav.fixed'), isNull);
    });

    testServer('ochena product → 404', (t) async {
      t.pumpComponent(site({}));
      expect((await t.request('/p/nai')).statusCode, 404);
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
