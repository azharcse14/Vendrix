import 'package:intl/intl.dart';
import 'package:shared/shared.dart';
import 'package:test/test.dart';

void main() {
  group('formatTaka (9.8, v1.12)', () {
    test('bn: Bangla digit, lakh comma, ৳ samne', () {
      expect(formatTaka(12500000, 'bn'), '৳১,২৫,০০০');
      expect(formatTaka(18050, 'bn'), '৳১৮০.৫০');
      expect(formatTaka(18000, 'bn'), '৳১৮০');
      expect(formatTaka(1, 'bn'), '৳০.০১');
      expect(formatTaka(0, 'bn'), '৳০');
      expect(formatTaka(100000000000, 'bn'), '৳১,০০,০০,০০,০০০');
    });
    test('en_IN: English digit, lakh comma', () {
      expect(formatTaka(12500000, 'en_IN'), '৳1,25,000');
      expect(formatTaka(18050, 'en_IN'), '৳180.50');
    });
    test('khub boro amount e poisha thik (double round na)', () {
      expect(formatTaka(900719925474099, 'en_IN'), '৳90,07,19,92,54,740.99');
    });
    test('NumberFormat.currency er output na (regression guard)', () {
      final cur = NumberFormat.currency(locale: 'bn', symbol: '৳').format(180);
      expect(formatTaka(18000, 'bn'), isNot(cur));
    });
    test('minus: chinho ৳ er age', () {
      expect(formatTaka(-18000, 'bn'), '-৳১৮০');
      expect(formatTaka(-18050, 'en_IN'), '-৳180.50');
    });
    test(
      'intl er currency pattern na (৳ pichone ba sobshomoy doshomik na)',
      () {
        for (final s in [formatTaka(18000, 'bn'), formatTaka(18000, 'en_IN')]) {
          expect(s.startsWith('৳'), isTrue, reason: s);
          expect(s.contains('.'), isFalse, reason: s);
        }
      },
    );
  });

  group('numberLocale (9.8, v1.12)', () {
    test('Bangla digit shudhu bn UI ar bangla_digits on', () {
      expect(numberLocale('bn', banglaDigits: true), 'bn');
      expect(numberLocale('bn', banglaDigits: false), 'en_IN');
      expect(numberLocale('en', banglaDigits: true), 'en_IN');
      expect(numberLocale('en', banglaDigits: false), 'en_IN');
    });
  });
}
