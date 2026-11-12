import 'package:intl/intl.dart';

/// Shonkhya er locale (9.8, v1.12): Bangla digit shudhu UI bhasha `bn` ar shop er
/// `locales.bangla_digits` on hole (`bn`), na hole `en_IN` (lakh comma, English digit).
String numberLocale(String uiLocale, {required bool banglaDigits}) =>
    uiLocale == 'bn' && banglaDigits ? 'bn' : 'en_IN';

/// Taka (poisha `int`) → `৳১,২৫,০০০`, `৳১৮০.৫০`, `-৳১৮০` (9.8). `locale` = [numberLocale].
/// `NumberFormat.currency` na: `bn` e ৳ pichone ar shob shomoy duita doshomik ashe.
String formatTaka(int poisha, String locale) {
  final abs = poisha.abs();
  var s = NumberFormat('#,##,##0', locale).format(abs ~/ 100);
  if (abs % 100 != 0) {
    // poisha alada int hishebe (double er round na); doshomik chinho locale er
    final f = NumberFormat('00', locale);
    s += '${f.symbols.DECIMAL_SEP}${f.format(abs % 100)}';
  }
  return '${poisha < 0 ? '-' : ''}৳$s';
}

/// Shonkhya (page, koyta product) locale er digit e, lakh comma shoho (9.8). `locale` = [numberLocale].
String formatCount(int n, String locale) =>
    NumberFormat('#,##,##0', locale).format(n);

/// Rating (gor, 0–5) ek doshomik e, locale er digit e: `৪.৫`, `4.5` (9.8). `locale` = [numberLocale].
String formatRating(double r, String locale) =>
    NumberFormat('0.0', locale).format(r);
