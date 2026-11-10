import 'package:shared/shared.dart';
import 'package:test/test.dart';

void main() {
  group('Translator fallback', () {
    const defaults = {
      'bn': {
        'a': 'ফাইল বাংলা',
        'b': 'শুধু বাংলা ফাইল',
        'c': 'বাংলা ফাইল সি',
        'hi': 'হাই {name}',
        'cart.items.other': '{count}টি পণ্য',
      },
      'en': {
        'a': 'file en',
        'c': 'file en c',
        'hi': 'Hi {name}, {name}!',
        'cart.items.one': '{count} item',
        'cart.items.other': '{count} items',
      },
    };
    const overrides = {
      'bn': {'b': 'বাংলা ওভাররাইড', 'd': 'শুধু বাংলা ওভাররাইড'},
      'en': {'c': 'override en c'},
    };
    late List<String> missing;
    Translator tr(String locale) {
      missing = [];
      return Translator(
        locale,
        defaults: defaults,
        overrides: overrides,
        onMissing: (k, l) => missing.add('$l:$k'),
      );
    }

    test(
      '5 dhap: override[locale] → file[locale] → override[bn] → file[bn] → key',
      () {
        final en = tr('en');
        expect(en.t('c'), 'override en c');
        expect(en.t('a'), 'file en');
        expect(en.t('d'), 'শুধু বাংলা ওভাররাইড');
        expect(en.t('b'), 'বাংলা ওভাররাইড');
        expect(en.t('nai'), 'nai');
        expect(missing, ['en:nai']);
        expect(tr('bn').t('a'), 'ফাইল বাংলা');
      },
    );
    test('placeholder', () {
      expect(tr('en').t('hi', {'name': 'Rahim'}), 'Hi Rahim, Rahim!');
      expect(tr('bn').t('hi', {'name': 'রহিম'}), 'হাই রহিম');
      expect(tr('en').t('hi'), 'Hi {name}, {name}!');
    });
    test('English plural .one/.other, count placeholder', () {
      expect(tr('en').plural('cart.items', 1), '1 item');
      expect(tr('en').plural('cart.items', 3), '3 items');
      // Bangla e ek-bochon/bohubochon alada na: shob shomoy .other
      expect(tr('bn').plural('cart.items', 1), '1টি পণ্য');
      expect(missing, isEmpty);
      // en e .one/.other kichui na: Bangla er .other
      final only = Translator(
        'en',
        defaults: const {
          'bn': {'x.other': '{count}টি'},
          'en': {},
        },
      );
      expect(only.plural('x', 1), '1টি');
    });
  });
}
