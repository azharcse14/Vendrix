import 'dart:convert';

import 'package:jaspr/server.dart';
import 'package:jaspr_test/server_test.dart';
import 'package:website/main.server.options.dart';

import '../fake_site.dart';

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  const shop = {
    'locales': {
      'enabled': ['bn', 'en'],
    },
    'branding': {
      'name': {'bn': 'সবুজ বাজার'},
    },
    'seo': {
      'title': {'bn': 'সবুজ বাজার: অর্গানিক খাবার'},
      'description': {'bn': 'খাঁটি খাবার'},
      'og_image_key': 'shops/1/og.jpg',
      'noindex': false,
    },
  };

  group('Meta ar Open Graph (10.4)', () {
    testServer('product: title, description (seo), OG property, dam, canonical, hreflang, JSON-LD', (t) async {
      t.pumpComponent(site(shop));
      final doc = (await t.request('/p/red-rice')).document!;
      String? prop(String p) => doc.querySelector('meta[property="$p"]')?.attributes['content'];
      expect(doc.querySelector('title')!.text, 'লাল চাল | সবুজ বাজার');
      expect(doc.querySelector('meta[name="description"]')!.attributes['content'], 'SEO বিবরণ');
      expect(doc.querySelector('meta[name="og:title"]'), isNull); // name na, property
      expect(prop('og:title'), 'লাল চাল');
      expect(prop('og:description'), 'SEO বিবরণ');
      expect(prop('og:type'), 'product');
      expect(prop('og:url'), 'https://shop.test/p/red-rice');
      expect(prop('og:image'), startsWith('https://img.test/'));
      expect(prop('og:image'), endsWith('shops/1/a.jpg'));
      expect(prop('product:price:amount'), '850.00');
      expect(prop('product:price:currency'), 'BDT');
      expect(doc.querySelector('link[rel="canonical"]')!.attributes['href'], 'https://shop.test/p/red-rice');
      final alt = {
        for (final l in doc.querySelectorAll('link[rel="alternate"]')) l.attributes['hreflang']: l.attributes['href'],
      };
      expect(alt, {
        'bn': 'https://shop.test/p/red-rice',
        'en': 'https://shop.test/en/p/red-rice',
        'x-default': 'https://shop.test/p/red-rice',
      });
      expect(doc.querySelector('meta[name="robots"]'), isNull);
      final ld = jsonDecode(doc.querySelector('script[type="application/ld+json"]')!.text) as Map<String, dynamic>;
      expect(ld['@type'], 'Product');
      expect(ld['name'], 'লাল চাল');
    });

    testServer('home: shop er seo title/description/og_image; category: naam, website type', (t) async {
      t.pumpComponent(site(shop));
      final home = (await t.request('/')).document!;
      expect(home.querySelector('title')!.text, 'সবুজ বাজার: অর্গানিক খাবার');
      expect(home.querySelector('meta[name="description"]')!.attributes['content'], 'খাঁটি খাবার');
      expect(home.querySelector('meta[property="og:type"]')!.attributes['content'], 'website');
      expect(home.querySelector('meta[property="og:image"]')!.attributes['content'], endsWith('shops/1/og.jpg'));
      expect(home.querySelector('link[rel="canonical"]')!.attributes['href'], 'https://shop.test/');

      final cat = (await t.request('/c/lal?page=2')).document!;
      expect(cat.querySelector('title')!.text, 'লাল চাল | সবুজ বাজার');
      expect(cat.querySelector('link[rel="canonical"]')!.attributes['href'], 'https://shop.test/c/lal');
      expect(cat.querySelector('meta[property="og:type"]')!.attributes['content'], 'website');
    });

    testServer('en bondho: hreflang shudhu bn ar x-default; seo.noindex: puro site noindex', (t) async {
      t.pumpComponent(
        site({
          ...shop,
          'locales': {
            'enabled': ['bn'],
          },
          'seo': {'noindex': true},
        }),
      );
      for (final path in ['/', '/p/red-rice', '/policy/terms']) {
        final doc = (await t.request(path)).document!;
        final langs = doc.querySelectorAll('link[rel="alternate"]').map((l) => l.attributes['hreflang']);
        expect(langs, unorderedEquals(['bn', 'x-default']), reason: path);
        expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex', reason: path);
        expect(doc.querySelectorAll('title'), hasLength(1), reason: path);
      }
    });
  });
}
