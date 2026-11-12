import 'package:test/test.dart';
import 'package:website/services/link.dart';

void main() {
  group('Admin er link (9.2, 12.2)', () {
    test('/ diye shuru: bhashar prefix shoho', () {
      expect(safeLink('/c/chal', 'bn'), '/c/chal');
      expect(safeLink('/c/chal', 'en'), '/en/c/chal');
      expect(safeLink('/', 'en'), '/en');
    });

    test('https:// host shoho hubohu', () {
      expect(safeLink('https://example.com/a?b=1', 'en'), 'https://example.com/a?b=1');
    });

    test('bipodjonok ba bhul: null', () {
      for (final bad in [
        null,
        '',
        '//evil.com',
        '/\\evil.com',
        '/\tx',
        '/a\nb',
        'javascript:alert(1)',
        'http://example.com',
        'https://',
        'https:///x',
        'c/chal',
        ' /c/chal',
      ]) {
        expect(safeLink(bad, 'bn'), isNull, reason: '$bad');
      }
    });
  });
}
