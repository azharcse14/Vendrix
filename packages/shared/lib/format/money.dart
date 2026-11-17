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

/// [formatTaka] er hubohu lekha, `intl` chhara: [digits] = `0`–`9` er locale er digit (jemon `০১২৩৪৫৬৭৮৯`), doshomik
/// `.`. Website er `@client` e (`intl` er number data JS e na ashe, 10.5).
String formatTakaDigits(int poisha, String digits) {
  final abs = poisha.abs();
  final whole = '${abs ~/ 100}';
  // Lakh comma: shesh 3, tarpor 2 kore.
  var head = whole.length > 3 ? whole.substring(0, whole.length - 3) : '';
  final groups = [whole.substring(head.length)];
  while (head.isNotEmpty) {
    final cut = head.length > 2 ? head.length - 2 : 0;
    groups.insert(0, head.substring(cut));
    head = head.substring(0, cut);
  }
  var s = groups.join(',');
  if (abs % 100 != 0) s += '.${'${abs % 100}'.padLeft(2, '0')}';
  final local = String.fromCharCodes([
    for (final c in s.codeUnits)
      c >= 0x30 && c <= 0x39 ? digits.codeUnitAt(c - 0x30) : c,
  ]);
  return '${poisha < 0 ? '-' : ''}৳$local';
}

/// Card er discount % (10.5 v1.38 item 2): `round((old − new) / old × 100)`, shudhu int (Rule 1: shudhu display,
/// taka hishab na). `compareAt` nai ba dam er cheye beshi na hole, ba 5% er kom hole `null` (badge na).
int? discountPercent(int price, int? compareAt) {
  if (compareAt == null || compareAt <= price) return null;
  final off = compareAt - price;
  if (off * 100 < compareAt * 5) return null;
  return (off * 200 + compareAt) ~/ (compareAt * 2);
}

final _takaInput = RegExp(r'^(\d{0,12})(?:\.(\d{1,2}))?$');

/// Form e lekha taka (`120.50`, `১২০.৫০`, `1,25,000`) → poisha `int`; string diye, `double` na. Khali, negative,
/// 2 er beshi doshomik ba bhul lekha hole `null` (admin form, Rule 1: hishab na, shudhu input).
int? parseTaka(String s) {
  final t = String.fromCharCodes([
    for (final c in s.trim().replaceAll(',', '').runes)
      c >= 0x09E6 && c <= 0x09EF ? c - 0x09E6 + 0x30 : c,
  ]);
  final m = _takaInput.firstMatch(t);
  if (m == null || t.isEmpty || t == '.') return null;
  final whole = m[1]!.isEmpty ? 0 : int.parse(m[1]!);
  return whole * 100 + int.parse((m[2] ?? '0').padRight(2, '0'));
}
