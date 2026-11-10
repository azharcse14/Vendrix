import 'dart:math' as math;

final _hex = RegExp(r'^#[0-9A-Fa-f]{6}$');

/// Admin er 5 rong (9.4): settings `theme.colors` er key.
const colorKeys = ['primary', 'accent', 'tint', 'text', 'background'];

List<int> _rgb(String hex) {
  if (!_hex.hasMatch(hex)) throw ArgumentError.value(hex, 'hex');
  return [
    for (var i = 1; i < 7; i += 2)
      int.parse(hex.substring(i, i + 2), radix: 16),
  ];
}

String _toHex(List<int> c) =>
    '#${c.map((v) => v.toRadixString(16).padLeft(2, '0')).join().toUpperCase()}';

/// sRGB channel e linear mix (9.3, v1.12): `t` = 0 hole `a`, 1 hole `b`; CSS `color-mix(in srgb)` er moto.
String mixColor(String a, String b, double t) {
  final x = _rgb(a), y = _rgb(b);
  return _toHex([
    for (var i = 0; i < 3; i++) (x[i] + (y[i] - x[i]) * t).round(),
  ]);
}

double _luminance(String hex) {
  final c = _rgb(hex).map((v) {
    final s = v / 255;
    return s <= 0.04045
        ? s / 12.92
        : math.pow((s + 0.055) / 1.055, 2.4).toDouble();
  }).toList();
  return 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
}

/// WCAG contrast (9.3): `(L_halka + 0.05) / (L_gaḍho + 0.05)`, 1 theke 21.
double contrastRatio(String a, String b) {
  final x = _luminance(a), y = _luminance(b);
  return (math.max(x, y) + 0.05) / (math.min(x, y) + 0.05);
}

/// Lekha r jonno je kono kom-contrast er seema (9.3).
const minContrast = 4.5;

String _onColor(String bg, String text) =>
    contrastRatio('#FFFFFF', bg) >= contrastRatio(text, bg) ? '#FFFFFF' : text;

/// 5 rong theke shob token (9.3 er table, v1.12 formula). Bhul ba nei rong e [ArgumentError].
Map<String, String> deriveTokens(Map<String, String> colors) {
  final c = {
    for (final k in colorKeys)
      k: colors[k] ?? (throw ArgumentError.value(colors, 'colors', '$k nai')),
  };
  c.values.forEach(_rgb);
  final bg = c['background']!, text = c['text']!;
  final surface = mixColor(bg, '#FFFFFF', 0.5);
  var muted = text;
  for (var p = 65; p <= 100; p++) {
    final m = mixColor(bg, text, p / 100);
    if (contrastRatio(m, bg) >= minContrast &&
        contrastRatio(m, surface) >= minContrast) {
      muted = m;
      break;
    }
  }
  return {
    'primary': c['primary']!,
    'primary-hover': mixColor(c['primary']!, '#000000', 0.08),
    'on-primary': _onColor(c['primary']!, text),
    'accent': c['accent']!,
    'on-accent': _onColor(c['accent']!, text),
    'tint': c['tint']!,
    'text': text,
    'text-muted': muted,
    'background': bg,
    'surface': surface,
    'border': mixColor(bg, text, 0.12),
  };
}

/// Kom contrast er ek jora (admin e warning `theme.low_contrast`).
class ContrastWarning {
  /// Lekha r token.
  final String foreground;

  /// Pichoner token.
  final String background;

  /// Contrast ratio (< [minContrast]).
  final double ratio;

  /// Warning.
  const ContrastWarning(this.foreground, this.background, this.ratio);
}

const _pairs = [
  ['text', 'background'],
  ['text', 'surface'],
  ['text', 'tint'],
  ['on-primary', 'primary'],
  ['on-accent', 'accent'],
  ['primary', 'background'],
];

/// [deriveTokens] er token e 9.3 er jora gulo (v1.12) [minContrast] er niche hole warning.
List<ContrastWarning> contrastWarnings(Map<String, String> tokens) => [
  for (final p in _pairs)
    if (contrastRatio(tokens[p[0]]!, tokens[p[1]]!) case final r
        when r < minContrast)
      ContrastWarning(p[0], p[1], r),
];

const _cssNames = {
  'primary': '--color-primary',
  'primary-hover': '--color-primary-hover',
  'on-primary': '--color-on-primary',
  'accent': '--color-accent',
  'on-accent': '--color-on-accent',
  'tint': '--color-tint',
  'text': '--color-text',
  'text-muted': '--color-text-muted',
  'background': '--color-bg',
  'surface': '--color-surface',
  'border': '--color-border',
};

final _font = RegExp(r'^[A-Za-z0-9 ]{1,60}$');

/// 9.3 er CSS variable (website `<head>` er `:root`): [deriveTokens] er rong, radius (px), font.
/// Font er naam admin er, tai shudhu harf, shonkhya, space (CSS e kichu dhukano na); bhul hole [ArgumentError].
Map<String, String> cssVariables(
  Map<String, String> tokens, {
  required int radius,
  required String headingFont,
  required String bodyFont,
}) {
  for (final f in [headingFont, bodyFont]) {
    if (!_font.hasMatch(f)) throw ArgumentError.value(f, 'font');
  }
  if (radius < 0 || radius > 64) throw ArgumentError.value(radius, 'radius');
  return {
    for (final e in _cssNames.entries)
      e.value:
          tokens[e.key] ??
          (throw ArgumentError.value(tokens, 'tokens', '${e.key} nai')),
    '--radius': '${radius}px',
    '--font-heading': '"$headingFont"',
    '--font-body': '"$bodyFont"',
  };
}
