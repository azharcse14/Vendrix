import 'package:test/test.dart';
import 'package:website/services/list_query.dart';

void main() {
  group('List er query (7.2, 10.1)', () {
    test('thik value: sort, in_stock, dam taka → poisha, page', () {
      final q = ListQuery.parse({'sort': 'price_asc', 'in_stock': '1', 'min': '100', 'max': '250', 'page': '3'});
      expect(q.sort, 'price_asc');
      expect(q.inStock, isTrue);
      expect(q.minPoisha, 10000);
      expect(q.maxPoisha, 25000);
      expect(q.page, 3);
    });

    test('bhul ba ochena value bad (API te 400 na)', () {
      final q = ListQuery.parse({'sort': 'x', 'in_stock': 'yes', 'min': '-5', 'max': '1e3', 'page': '0'});
      expect(q.sort, isNull);
      expect(q.inStock, isFalse);
      expect(q.minPoisha, isNull);
      expect(q.maxPoisha, isNull);
      expect(q.page, 1);
      expect(ListQuery.parse({'page': '99999999999999999999'}).page, 1);
      expect(ListQuery.parse({'max': '99999999999999'}).maxPoisha, isNull);
    });

    test('page link: baki filter thake, page 1 e page na', () {
      final q = ListQuery.parse({'sort': 'newest', 'in_stock': '1', 'min': '100', 'page': '2', 'utm': 'x'});
      expect(q.withPage(3), {'sort': 'newest', 'in_stock': '1', 'min': '100', 'page': '3'});
      expect(q.withPage(1), {'sort': 'newest', 'in_stock': '1', 'min': '100'});
    });

    test('search: q trim, 100 okkhor (API er shima), khali hole null, page link e thake', () {
      expect(ListQuery.parse({'q': '  লাল চাল '}).q, 'লাল চাল');
      expect(ListQuery.parse({'q': '   '}).q, isNull);
      expect(ListQuery.parse({'q': 'চ' * 150}).q!.runes, hasLength(100));
      expect(ListQuery.parse({'q': 'x', 'page': '2'}).withPage(1), {'q': 'x'});
    });
  });
}
