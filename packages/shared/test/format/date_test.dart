import 'package:shared/shared.dart';
import 'package:test/test.dart';

void main() {
  group('formatDate (9.8, Rule 5, v1.12)', () {
    // UTC 26 October 20:00 = Dhaka 27 October 02:00
    final utc = DateTime.utc(2026, 10, 26, 20);
    test('shop er timezone e din', () {
      expect(
        formatDate(utc, uiLocale: 'bn', banglaDigits: true),
        '২৭ অক্টোবর ২০২৬',
      );
      expect(
        formatDate(
          DateTime.utc(2026, 10, 26, 17, 59),
          uiLocale: 'bn',
          banglaDigits: true,
        ),
        '২৬ অক্টোবর ২০২৬',
      );
    });
    test('mash UI bhasha theke, digit numberLocale theke', () {
      expect(
        formatDate(utc, uiLocale: 'bn', banglaDigits: false),
        '27 অক্টোবর 2026',
      );
      expect(
        formatDate(utc, uiLocale: 'en', banglaDigits: true),
        '27 October 2026',
      );
    });
    test('local DateTime o UTC hishebe dhore', () {
      expect(
        formatDate(utc.toLocal(), uiLocale: 'en', banglaDigits: false),
        '27 October 2026',
      );
    });
    test('ochena timezone e error', () {
      expect(
        () => formatDate(
          utc,
          uiLocale: 'bn',
          banglaDigits: true,
          timeZone: 'Asia/Kolkata',
        ),
        throwsArgumentError,
      );
    });
  });
}
