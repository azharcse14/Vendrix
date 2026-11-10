import 'dart:convert';

import 'package:shared/shared.dart';
import 'package:test/test.dart';

// M1 er local API theke neya JSON (catalog.md)
const listJson =
    '{"items":[{"id":1,"slug":"lal-chal","name":"লাল চাল","unit":"kg","variable_weight":true,"price":42000,'
    '"compare_at_price":45000,"in_stock":true,"rating":4.5,"review_count":4,"image":"shops/1/a.jpg"},'
    '{"id":2,"slug":"modhu","name":"মধু","unit":"piece","variable_weight":false,"price":50000,'
    '"compare_at_price":null,"in_stock":false,"rating":0,"review_count":0,"image":null}],'
    '"page":2,"limit":24,"total":26}';
const productJson =
    '{"id":1,"slug":"lal-chal","name":"Red Rice","description":"About","unit":"piece","variable_weight":false,'
    '"category":"chal","attributes":{"origin":"Dinajpur"},"seo":{"title":"Title"},"rating":0,"review_count":0,'
    '"variants":[{"id":1,"sku":"LAL-5","options":{"weight":"5 kg"},"price":42000,"compare_at_price":null,'
    '"nominal_weight_g":5000,"in_stock":true}],'
    '"images":[{"storage_key":"shops/1/1.jpg","alt":"এক","width":10,"height":20}]}';
const categoriesJson =
    '[{"id":1,"slug":"chal","name":"চাল","image_key":null,"children":[{"id":2,"slug":"lal-chal","name":"লাল চাল","image_key":"k.jpg"}]}]';

void main() {
  group('Catalog model (7.1, 7.2, M1 er response)', () {
    test('Page<ProductCard>', () {
      final p = Page.fromJson(
        jsonDecode(listJson) as Map<String, dynamic>,
        ProductCard.fromJson,
      );
      expect(
        [p.page, p.limit, p.total, p.items.length, p.hasNext],
        [2, 24, 26, 2, false],
      );
      final a = p.items.first;
      expect(
        [
          a.id,
          a.slug,
          a.name,
          a.unit,
          a.variableWeight,
          a.price,
          a.compareAtPrice,
          a.inStock,
          a.rating,
          a.reviewCount,
          a.image,
        ],
        [
          1,
          'lal-chal',
          'লাল চাল',
          'kg',
          true,
          42000,
          45000,
          true,
          4.5,
          4,
          'shops/1/a.jpg',
        ],
      );
      expect(
        [p.items[1].compareAtPrice, p.items[1].image, p.items[1].rating],
        [null, null, 0.0],
      );
    });
    test('Product detail', () {
      final p = Product.fromJson(
        jsonDecode(productJson) as Map<String, dynamic>,
      );
      expect(
        [
          p.name,
          p.description,
          p.category,
          p.attributes['origin'],
          p.seo['title'],
        ],
        ['Red Rice', 'About', 'chal', 'Dinajpur', 'Title'],
      );
      final v = p.variants.single;
      expect(
        [
          v.id,
          v.sku,
          v.options['weight'],
          v.price,
          v.compareAtPrice,
          v.nominalWeightG,
          v.inStock,
        ],
        [1, 'LAL-5', '5 kg', 42000, null, 5000, true],
      );
      final i = p.images.single;
      expect(
        [i.storageKey, i.alt, i.width, i.height],
        ['shops/1/1.jpg', 'এক', 10, 20],
      );
    });
    test('Category tree', () {
      final c = (jsonDecode(categoriesJson) as List<dynamic>)
          .map((e) => Category.fromJson(e as Map<String, dynamic>))
          .toList();
      expect(
        [
          c.single.slug,
          c.single.imageKey,
          c.single.children.single.name,
          c.single.children.single.children,
        ],
        ['chal', null, 'লাল চাল', isEmpty],
      );
    });
    test('bhul type e FormatException', () {
      expect(() => ProductCard.fromJson({'id': 'x'}), throwsFormatException);
    });
  });

  group('PublicSettings (7.2, 9.2)', () {
    test('theme, locale, payment', () {
      final s = PublicSettings.fromJson({
        'branding': {
          'name': {'bn': 'সবুজ বাজার', 'en': 'Sobuj Bazar'},
        },
        'theme': {
          'preset': 'organic',
          'colors': {'primary': '#1F5A32'},
          'radius': 14,
        },
        'locales': {
          'enabled': ['bn', 'en'],
          'default': 'bn',
          'bangla_digits': true,
        },
        'checkout': {
          'payment_methods': ['cod'],
        },
      });
      expect(s.name.of('en'), 'Sobuj Bazar');
      expect(
        [
          s.colors['primary'],
          s.defaultLocale,
          s.banglaDigits,
          s.enabledLocales,
          s.paymentMethods,
        ],
        [
          '#1F5A32',
          'bn',
          true,
          ['bn', 'en'],
          ['cod'],
        ],
      );
      expect(s.raw['theme'], isA<Map<String, dynamic>>());
    });
    test('block na thakle default', () {
      final s = PublicSettings.fromJson({});
      expect(
        [
          s.defaultLocale,
          s.banglaDigits,
          s.enabledLocales,
          s.colors,
          s.name.of('bn'),
        ],
        [
          'bn',
          true,
          ['bn'],
          <String, String>{},
          '',
        ],
      );
    });
  });
}
