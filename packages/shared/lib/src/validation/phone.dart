final _format = RegExp(r'\p{Cf}', unicode: true);
final _separator = RegExp(r'[\s\p{Pd}.()−]', unicode: true);

/// Bangladesh mobile → `+8801XXXXXXXXX`; bhul hole `null` (8.1). Niyom
/// `backend/internal/phone` er `Normalize` er hubohu (Go-i spec; bodlale duitai, ekoi test case).
String? normalizePhone(String input) {
  final b = StringBuffer();
  var plus = false;
  for (final r in input.runes) {
    final c = String.fromCharCode(r);
    if (r >= 0x30 && r <= 0x39) {
      b.write(c);
    } else if (r >= 0x09E6 && r <= 0x09EF) {
      b.write(r - 0x09E6); // Bangla shonkhya
    } else if (c == '+') {
      if (plus || b.isNotEmpty) return null;
      plus = true;
    } else if (_format.hasMatch(c) || _separator.hasMatch(c)) {
      // lukano format chinho ba separator, bad
    } else {
      return null;
    }
  }
  var d = b.toString();
  if (plus && !d.startsWith('880')) return null;
  if (d.startsWith('00880')) d = d.substring(2); // international prefix
  if (d.startsWith('880')) d = d.substring(3);
  if (d.length == 10 && d[0] == '1') d = '0$d';
  if (d.length != 11 || d[0] != '0' || d[1] != '1' || d.codeUnitAt(2) < 0x33) {
    return null;
  }
  return '+88$d';
}
