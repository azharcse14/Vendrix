import 'package:test/test.dart';
import 'package:website/services/locale.dart';

void main() {
  group('URL theke bhasha (9.8, 10.3)', () {
    for (final (path, locale, rest) in [
      ('/', 'bn', '/'),
      ('/c/chal', 'bn', '/c/chal'),
      ('/en', 'en', '/'),
      ('/en/', 'en', '/'),
      ('/en/p/lal-chal', 'en', '/p/lal-chal'),
      ('/english', 'bn', '/english'),
      ('/c/en', 'bn', '/c/en'),
    ]) {
      test('$path → $locale $rest', () {
        final r = localeOf(path);
        expect(r.locale, locale);
        expect(r.path, rest);
      });
    }
  });
}
