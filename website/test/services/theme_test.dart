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

    test(':root e v1.38 er notun variable', () {
      final css = themeCss(settings('Baloo Da 2', 'Hind Siliguri'));
      expect(css, contains('--color-primary-soft:'));
      expect(css, contains('--color-focus:#1F5A32;'));
      expect(css, contains('--color-text-rgb:59 42 26;'));
      expect(css, contains('--radius-sm:7px;'));
      expect(css, contains('--shadow-2:'));
    });

    test('@font-face: body shudhu 400, heading 500/600, swap, nijer server theke (9.6 v1.38)', () {
      final css = themeCss(settings('Baloo Da 2', 'Hind Siliguri'));
      final faces = RegExp('@font-face').allMatches(css).length;
      expect(faces, (1 + 2) * 2); // body 400 + heading 500/600, × bengali/latin
      expect(css, contains('src:url(/fonts/hind-siliguri-bengali-400.woff2) format("woff2")'));
      expect(css, isNot(contains('hind-siliguri-bengali-500')));
      expect(css, isNot(contains('hind-siliguri-latin-600')));
      expect(css, contains('src:url(/fonts/baloo-da-2-bengali-500.woff2) format("woff2")'));
      expect(css, contains('src:url(/fonts/baloo-da-2-bengali-600.woff2) format("woff2")'));
      expect(css, isNot(contains('baloo-da-2-bengali-400')));
      expect(css, contains('font-display:swap'));
      expect(css, contains('unicode-range:U+0951-0952'));
      expect(css, isNot(contains('noto-sans')));
      expect(css, isNot(contains('http')));
    });

    test('heading ar body ekoi font hole 400/500/600 ek shathe', () {
      final css = themeCss(settings('Hind Siliguri', 'Hind Siliguri'));
      expect(RegExp('@font-face').allMatches(css).length, 3 * 2);
    });

    test('Tiro Bangla shudhu 400 (heading e o); ek font duibar na', () {
      expect(RegExp('@font-face').allMatches(themeCss(settings('Tiro Bangla', 'Tiro Bangla'))).length, 2);
      final css = themeCss(settings('Tiro Bangla', 'Hind Siliguri'));
      expect(RegExp('@font-face').allMatches(css).length, 4);
      expect(css, contains('tiro-bangla-bengali-400'));
    });

    test('ochena font e @font-face na (variable thake)', () {
      final css = themeCss(settings('Comic Sans', 'Hind Siliguri'));
      expect(RegExp('@font-face').allMatches(css).length, 2);
    });

    test('preload shudhu body font er bengali 400', () {
      expect(fontPreload(settings('Baloo Da 2', 'Hind Siliguri')), '/fonts/hind-siliguri-bengali-400.woff2');
      expect(fontPreload(settings('Baloo Da 2', 'Comic Sans')), isNull);
    });

    test('Tailwind font fallback stack (9.6 v1.38)', () {
      final config = File('tailwind.config.js').readAsStringSync();
      const stack = '\'"Noto Sans Bengali"\', \'"Nirmala UI"\', \'sans-serif\'';
      expect(config, contains("heading: ['var(--font-heading)', $stack]"));
      expect(config, contains("body: ['var(--font-body)', $stack]"));
    });

    test('protiti font er file ache', () {
      for (final f in fontFiles()) {
        expect(File('web/fonts/$f').existsSync(), isTrue, reason: f);
      }
      expect(fontFiles(), hasLength(5 * 3 * 2 + 2));
    });
  });
}
