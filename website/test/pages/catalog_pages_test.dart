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
}
