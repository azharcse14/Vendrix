import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';
import 'package:shelf/shelf.dart';
import 'package:test/test.dart';
import 'package:website/seo/sitemap.dart';
import 'package:website/services/settings_cache.dart';

void main() {
  const json = {'content-type': 'application/json'};
  final pages = <String>[];
  final api = MockClient((r) async {
    if (r.url.path == '/api/categories') {
      return http.Response(
        jsonEncode([
          {
            'id': 1,
            'slug': 'chal',
            'name': 'চাল',
            'image_key': null,
            'children': [
              {'id': 2, 'slug': 'lal', 'name': 'লাল', 'image_key': null, 'children': []},
            ],
          },
        ]),
        200,
        headers: json,
      );
    }
    final page = int.parse(r.url.queryParameters['page']!);
    pages.add('${r.url.queryParameters['limit']}/$page');
    return http.Response(
      jsonEncode({
        'items': [
          {
            'id': page,
            'slug': 'p$page',
            'name': 'x',
            'unit': 'kg',
            'variable_weight': false,
            'price': 1,
            'compare_at_price': null,
            'in_stock': true,
            'rating': 0,
            'review_count': 0,
            'image': null,
          },
        ],
        'page': page,
        'limit': 100,
        'total': 150,
      }),
      200,
      headers: json,
    );
  });
  final catalog = CatalogRepository(ApiClient(Uri.parse('http://api.test'), api));

  group('sitemap ar robots (10.4)', () {
    test('home, category (child shoho), product (shob page), policy; en chalu hole /en o', () async {
      pages.clear();
      final xml = await sitemapXml(catalog, origin: 'https://shop.test', english: true);
      final locs = RegExp('<loc>([^<]*)</loc>').allMatches(xml).map((m) => m[1]).toList();
      for (final p in ['/', '/c/chal', '/c/lal', '/p/p1', '/p/p2', '/policy/terms', '/policy/privacy']) {
        expect(locs, contains('https://shop.test$p'), reason: p);
        expect(locs, contains(p == '/' ? 'https://shop.test/en' : 'https://shop.test/en$p'), reason: p);
      }
      expect(pages, ['100/1', '100/2']);
      expect(xml, startsWith('<?xml version="1.0" encoding="UTF-8"?>'));

      final bn = await sitemapXml(catalog, origin: 'https://shop.test', english: false);
      expect(bn, isNot(contains('/en')));
    });

    test('robots: 10.4 er path block (duita bhasha), sitemap er link', () {
      final txt = robotsTxt('https://shop.test');
      for (final p in ['/search', '/cart', '/checkout', '/account', '/order', '/confirm', '/print']) {
        expect(txt, contains('Disallow: $p\n'));
        expect(txt, contains('Disallow: /en$p\n'));
      }
      expect(txt, contains('Sitemap: https://shop.test/sitemap.xml'));
    });

    test('handler: host er nijer sitemap; ochena shop 404; onno path page e jay', () async {
      final handler = seoFiles(
        (host) async => host == 'shop.test'
            ? const ShopSettings(PublicSettings({}), 1, {})
            : throw const ApiError(404, 'shop_not_found'),
        (request) => catalog,
      )((_) => Response.ok('page'));
      Future<Response> get(String path, String host) =>
          handler(Request('GET', Uri.parse('http://$host$path'), headers: {'host': host}));

      final sm = await get('/sitemap.xml', 'shop.test');
      expect(sm.statusCode, 200);
      expect(sm.headers['content-type'], startsWith('application/xml'));
      expect(await sm.readAsString(), contains('<loc>http://shop.test/c/chal</loc>'));
      final robots = await get('/robots.txt', 'shop.test');
      expect(robots.headers['content-type'], startsWith('text/plain'));
      expect((await get('/sitemap.xml', 'nai.test')).statusCode, 404);
      expect(await (await get('/c/chal', 'shop.test')).readAsString(), 'page');
    });
  });
}
