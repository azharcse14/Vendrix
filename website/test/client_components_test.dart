import 'dart:io';

import 'package:shared/shared.dart';
import 'package:test/test.dart';
import 'package:website/components/checkout_form.dart';

void main() {
  group('@client shudhu 10.2 er list', () {
    test(
      'lib/ e @client thaka file shudhu filter sheet, login form (v1.19), variant picker, add to cart ar wishlist '
      'button (v1.28), checkout form (v1.32)',
      () {
        final files = [
          for (final f in Directory('lib').listSync(recursive: true).whereType<File>())
            if (f.path.endsWith('.dart') && RegExp(r'^@client\b', multiLine: true).hasMatch(f.readAsStringSync()))
              f.path.replaceAll(r'\', '/'),
        ]..sort();
        expect(files, [
          'lib/components/add_to_cart.dart',
          'lib/components/checkout_form.dart',
          'lib/components/filter_sheet.dart',
          'lib/components/login_form.dart',
          'lib/components/variant_picker.dart',
          'lib/components/wishlist_button.dart',
        ]);
      },
    );
  });

  group('CheckoutForm (10.13 item 1)', () {
    test('takaText (client, intl chhara) server er formatTaka er hubohu', () {
      for (final p in [0, 5, 100, 6000, 18050, 170000, 12500000, 123456789, -18000]) {
        expect(takaText(p, '০১২৩৪৫৬৭৮৯'), formatTaka(p, 'bn'), reason: '$p');
        expect(takaText(p, '0123456789'), formatTaka(p, 'en_IN'), reason: '$p');
      }
    });

    test('Idempotency-Key: 128 bit base32 (26 okkhor, Go er ^[A-Za-z0-9_-]{16,64}\$), protibar notun', () {
      final keys = {for (var i = 0; i < 50; i++) newIdempotencyKey()};
      expect(keys, hasLength(50));
      for (final k in keys) {
        expect(k, matches(RegExp(r'^[A-Z2-7]{26}$')));
      }
    });

    test('IdempotencyKeys: ekoi body te ekoi key (retry, OTP), body bodlale notun key (8.6 step 1)', () {
      final keys = IdempotencyKeys();
      final a = {'note': 'x', 'expected_total': 100};
      final k1 = keys.keyFor(a);
      expect(keys.keyFor({'note': 'x', 'expected_total': 100}), k1);
      final k2 = keys.keyFor({'note': 'x', 'expected_total': 200});
      expect(k2, isNot(k1));
      expect(keys.keyFor({'note': 'y', 'expected_total': 200}), isNot(k2));
    });
  });
}
