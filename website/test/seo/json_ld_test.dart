import 'dart:convert';

import 'package:shared/shared.dart';
import 'package:test/test.dart';
import 'package:website/seo/json_ld.dart';

Product product({String name = 'চাল', int reviews = 0}) => Product(
  1,
  'chal',
  name,
  'ভালো চাল',
  'kg',
  false,
  null,
  const {},
  const {},
  4.5,
  reviews,
  const [Variant(7, 'R7', {}, 18050, null, null, true), Variant(8, 'R8', {}, 85000, null, null, false)],
  const [],
);

void main() {
  group('JSON-LD escape (10.4, 12.2)', () {
    test('naam e </script> thakleo < ashe na, JSON thik', () {
      const evil = '</script><script>alert(1)</script>';
      final out = jsonLd(
        productJsonLd(
          product(name: evil),
          url: 'https://s.test/p/chal',
          images: const [],
        ),
      );
      expect(out, isNot(contains('<')));
      expect((jsonDecode(out) as Map)['name'], evil);
    });

    test('Product: offer protiti variant er (dam taka, BDT, stock), aggregateRating shudhu review thakle', () {
      final ld = productJsonLd(product(), url: 'https://s.test/p/chal', images: const ['https://img/a.jpg']);
      expect(ld['@context'], 'https://schema.org');
      expect(ld['image'], ['https://img/a.jpg']);
      final offers = ld['offers'] as List;
      expect(offers.first, {
        '@type': 'Offer',
        'sku': 'R7',
        'price': '180.50',
        'priceCurrency': 'BDT',
        'availability': 'https://schema.org/InStock',
        'url': 'https://s.test/p/chal',
      });
      expect((offers.last as Map)['availability'], 'https://schema.org/OutOfStock');
      expect(ld.containsKey('aggregateRating'), isFalse);
      final rated = productJsonLd(product(reviews: 3), url: 'u', images: const []);
      expect(rated['aggregateRating'], {'@type': 'AggregateRating', 'ratingValue': 4.5, 'reviewCount': 3});
    });

    test('taka: poisha → doshomik lekha (hishab na)', () {
      expect(amount(85000), '850.00');
      expect(amount(18005), '180.05');
    });
  });
}
