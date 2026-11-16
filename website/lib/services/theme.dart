import 'package:shared/shared.dart';

// 9.6 er font: naam → (`web/fonts/` er file er naam, weight). Fontsource 5.3.0 theke (OFL, LICENSE-*.txt).
const _fonts = {
  'Hind Siliguri': ('hind-siliguri', [400, 500, 600]),
  'Baloo Da 2': ('baloo-da-2', [400, 500, 600]),
  'Tiro Bangla': ('tiro-bangla', [400]),
  'Noto Sans Bengali': ('noto-sans-bengali', [400, 500, 600]),
  'Noto Serif Bengali': ('noto-serif-bengali', [400, 500, 600]),
  'Anek Bangla': ('anek-bangla', [400, 500, 600]),
};

// Browser shudhu dorkari subset er file ane (unicode-range).
const _subsets = {
  'bengali':
      'U+0951-0952,U+0964-0965,U+0980-09FE,U+1CD0,U+1CD2,U+1CD5-1CD6,U+1CD8,U+1CE1,U+1CEA,U+1CED,U+1CF2,'
      'U+1CF5-1CF7,U+200C-200D,U+20B9,U+25CC,U+A8F1',
  'latin':
      'U+0000-00FF,U+0131,U+0152-0153,U+02BB-02BC,U+02C6,U+02DA,U+02DC,U+0304,U+0308,U+0329,U+2000-206F,'
      'U+20AC,U+2122,U+2191,U+2193,U+2212,U+2215,U+FEFF,U+FFFD',
};

/// `<head>` er `<style>` (9.3, 9.6): `:root` e settings er rong, radius, font; `@font-face` (`font-display: swap`)
/// body font e shudhu 400, heading font e 500/600 (na thakle 400). Bhul rong ba font naam e `ArgumentError` (`cssVariables`).
String themeCss(PublicSettings s) {
  final vars = cssVariables(
    deriveTokens(s.colors),
    radius: s.radius,
    headingFont: s.headingFont,
    bodyFont: s.bodyFont,
  );
  final out = StringBuffer(':root{');
  vars.forEach((k, v) => out.write('$k:$v;'));
  out.write('}');
  // 9.6 v1.38: prothom view e kom file; emphasis heading font diye.
  final want = <String, Set<int>>{};
  void add(String family, Set<int> ws) {
    final have = _fonts[family]?.$2 ?? const <int>[];
    final pick = have.where(ws.contains).toSet();
    want.putIfAbsent(family, () => {}).addAll(pick.isEmpty ? have.take(1) : pick);
  }

  add(s.bodyFont, {400});
  add(s.headingFont, {500, 600});
  for (final MapEntry(key: family, value: weights) in want.entries) {
    final id = _fonts[family]?.$1;
    for (final w in weights.toList()..sort()) {
      for (final e in _subsets.entries) {
        out.write(
          '@font-face{font-family:"$family";font-style:normal;font-weight:$w;font-display:swap;'
          'src:url(/fonts/$id-${e.key}-$w.woff2) format("woff2");unicode-range:${e.value}}',
        );
      }
    }
  }
  return out.toString();
}

/// `<link rel=preload>` er font (9.6 v1.38): shudhu body font er bengali 400; ochena font e `null`.
String? fontPreload(PublicSettings s) {
  final id = _fonts[s.bodyFont]?.$1;
  return id == null ? null : '/fonts/$id-bengali-400.woff2';
}

/// `web/fonts/` e je file thakar kotha (test).
List<String> fontFiles() => [
  for (final (id, weights) in _fonts.values)
    for (final w in weights)
      for (final sub in _subsets.keys) '$id-$sub-$w.woff2',
];
