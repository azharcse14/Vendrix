final _control = RegExp(r'[\x00-\x1f\x7f\\]');

/// Admin er link (banner, 9.2) render er age abar check (12.2): `/...` (kintu `//` na, `\`, tab, newline ba
/// control character na; browser `/\evil.com` ke `//evil.com` dhore) → ei bhashar link; `https://` + host → hubohu;
/// baki `null` (link chhara dekhao).
String? safeLink(String? raw, String locale) {
  if (raw == null || raw.isEmpty || _control.hasMatch(raw)) return null;
  if (raw.startsWith('https://')) {
    final u = Uri.tryParse(raw);
    return u != null && u.host.isNotEmpty ? raw : null;
  }
  if (!raw.startsWith('/') || raw.startsWith('//')) return null;
  if (locale != 'en') return raw;
  return raw == '/' ? '/en' : '/en$raw';
}
