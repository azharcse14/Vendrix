import 'dart:io';

import 'package:shared/shared.dart';
import 'package:test/test.dart';
import 'package:website/services/theme.dart';

void main() {
  PublicSettings settings(String heading, String body) => PublicSettings.fromJson({
    'theme': {
      'colors': {
        'primary': '#1F5A32',
        'accent': '#F2C230',
        'tint': '#E3EED3',
        'text': '#3B2A1A',
        'background': '#FBFBF6',
      },
      'fonts': {'heading': heading, 'body': body},
      'radius': 14,
    },
  });

  group('Theme CSS (9.3, 9.6)', () {
    test(':root e settings er variable', () {
      final css = themeCss(settings('Baloo Da 2', 'Hind Siliguri'));
      expect(css, startsWith(':root{'));
      expect(css, contains('--color-primary:#1F5A32;'));
      expect(css, contains('--radius:14px;'));
      expect(css, contains('--font-heading:"Baloo Da 2";'));
    });

    test('@font-face shudhu duita font er, swap, nijer server theke', () {
      final css = themeCss(settings('Baloo Da 2', 'Hind Siliguri'));
      final faces = RegExp('@font-face').allMatches(css).length;
      expect(faces, 2 * 3 * 2); // 2 font × 400/500/600 × bengali/latin
      expect(css, contains('src:url(/fonts/baloo-da-2-bengali-600.woff2) format("woff2")'));
      expect(css, contains('font-display:swap'));
      expect(css, contains('unicode-range:U+0951-0952'));
      expect(css, isNot(contains('noto-sans')));
      expect(css, isNot(contains('http')));
    });

    test('Tiro Bangla shudhu 400; ek font duibar na', () {
      final css = themeCss(settings('Tiro Bangla', 'Tiro Bangla'));
      expect(RegExp('@font-face').allMatches(css).length, 2);
    });

    test('ochena font e @font-face na (variable thake)', () {
      final css = themeCss(settings('Comic Sans', 'Hind Siliguri'));
      expect(RegExp('@font-face').allMatches(css).length, 6);
    });

    test('protiti font er file ache', () {
      for (final f in fontFiles()) {
        expect(File('web/fonts/$f').existsSync(), isTrue, reason: f);
      }
      expect(fontFiles(), hasLength(5 * 3 * 2 + 2));
    });
  });
}
