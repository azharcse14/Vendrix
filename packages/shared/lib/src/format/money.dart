import 'package:intl/intl.dart';

/// Shonkhya er locale (9.8, v1.12): Bangla digit shudhu UI bhasha `bn` ar shop er
/// `locales.bangla_digits` on hole (`bn`), na hole `en_IN` (lakh comma, English digit).
String numberLocale(String uiLocale, {required bool banglaDigits}) =>
    uiLocale == 'bn' && banglaDigits ? 'bn' : 'en_IN';

/// Taka (poisha `int`) → `৳১,২৫,০০০`, `৳১৮০.৫০`, `-৳১৮০` (9.8). `locale` = [numberLocale].
/// `NumberFormat.currency` na: `bn` e ৳ pichone ar shob shomoy duita doshomik ashe.
String formatTaka(int poisha, String locale) {
  final abs = poisha.abs();
  final s = abs % 100 == 0
      ? NumberFormat('#,##,##0', locale).format(abs ~/ 100)
      : NumberFormat('#,##,##0.00', locale).format(abs / 100);
  return '${poisha < 0 ? '-' : ''}৳$s';
}
