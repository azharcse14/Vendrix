import 'package:shared/shared.dart';
import 'package:test/test.dart';

void main() {
  group('PublicSettings (9.2)', () {
    test('theme, branding er field', () {
      final s = PublicSettings.fromJson({
        'branding': {
          'name': {'bn': 'সবুজ বাজার'},
          'logo_key': 'shops/1/logo.png',
          'phone': '+8801712345678',
          'email': 'hello@example.com',
          'dbid_number': 'DBID-1',
          'bfsa_number': null,
        },
        'theme': {
          'fonts': {'heading': 'Baloo Da 2', 'body': 'Hind Siliguri'},
          'radius': 14,
        },
        'homepage': {
          'announcement': {'bn': 'ফ্রি ডেলিভারি'},
        },
      });
      expect(s.headingFont, 'Baloo Da 2');
      expect(s.bodyFont, 'Hind Siliguri');
      expect(s.radius, 14);
      expect(s.logoKey, 'shops/1/logo.png');
      expect(s.phone, '+8801712345678');
      expect(s.email, 'hello@example.com');
      expect(s.dbidNumber, 'DBID-1');
      expect(s.bfsaNumber, isNull);
      expect(s.announcement.of('en'), 'ফ্রি ডেলিভারি');
    });

    test('nai ba bhul type e default (Hind Siliguri, radius 8, null)', () {
      final s = PublicSettings.fromJson({
        'theme': {'fonts': 'x', 'radius': '14'},
        'branding': {'phone': 5},
      });
      expect(s.headingFont, 'Hind Siliguri');
      expect(s.bodyFont, 'Hind Siliguri');
      expect(s.radius, 8);
      expect(s.phone, isNull);
      expect(s.logoKey, isNull);
      expect(s.announcement.of('bn'), '');
    });
  });
}
