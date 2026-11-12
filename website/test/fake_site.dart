// Page test er fake Go API (7.2 er shape) ar Site.
import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';
import 'package:website/app.dart';
import 'package:website/components/site.dart';
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

/// Fake API te jawa URL (test clear kore).
final requests = <Uri>[];

const _json = {'content-type': 'application/json; charset=utf-8'};
final fakeApi = MockClient((r) async {
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
        'seo': {'description': 'SEO বিবরণ'},
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
        headers: _json,
      );
  }
  return http.Response(jsonEncode(body), 200, headers: _json);
});

/// Fake shop (`https://shop.test`), settings [settings].
Component site(Map<String, dynamic> settings) => Site(
  origin: 'https://shop.test',
  locale: 'bn',
  settings: PublicSettings(settings),
  translator: const Translator('bn'),
  catalog: CatalogRepository(ApiClient(Uri.parse('http://api.test'), fakeApi)),
  images: ImageUrls(Uri.parse('https://img.test'), resize: true),
  child: const App(),
);
