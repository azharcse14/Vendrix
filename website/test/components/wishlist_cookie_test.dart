import 'package:test/test.dart';
import 'package:website/components/wishlist_button.dart';

void main() {
  group('Guest wishlist cookie wl (10.3, 10.12 item 3)', () {
    test('pora: koma diye decimal ID, bhul ba 0 bad, onno cookie thakleo', () {
      expect(wlIds('a=1; wl=3,x,0,-2,5,3; b=2'), [3, 5]);
      expect(wlIds('wl='), isEmpty);
      expect(wlIds(''), isEmpty);
    });

    test('lekha: koma literal (%2C na, BFF koma diye bhag kore), max 50 (notun ta thake), 1 bochor, Secure, Lax', () {
      final ids = [for (var i = 1; i <= 51; i++) i];
      final c = wlCookie(ids);
      expect(c, startsWith('wl=${[for (var i = 2; i <= 51; i++) i].join(',')};'));
      expect(c, isNot(contains('%2C')));
      expect(c, contains('; Path=/; Max-Age=31536000; Secure; SameSite=Lax'));
      expect(c.toLowerCase(), isNot(contains('httponly')));
      expect(wlCookie([]), 'wl=; Path=/; Max-Age=0; Secure; SameSite=Lax');
    });
  });

  group('Session expire e login (10.12 item 7)', () {
    test('loginHref: ekhon er path next e, /en hole /en/login', () {
      expect(loginHref('/p/lal-chal'), '/login?next=%2Fp%2Flal-chal');
      expect(loginHref('/en/p/lal-chal'), '/en/login?next=%2Fen%2Fp%2Flal-chal');
      expect(loginHref('/en'), '/en/login?next=%2Fen');
    });
  });
}
