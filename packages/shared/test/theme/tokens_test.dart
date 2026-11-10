import 'package:shared/shared.dart';
import 'package:test/test.dart';

const presets = {
  'organic': {
    'primary': '#1F5A32',
    'accent': '#F2C230',
    'tint': '#E3EED3',
    'text': '#3B2A1A',
    'background': '#FBFBF6',
  },
  'fashion': {
    'primary': '#2A3480',
    'accent': '#B83A5E',
    'tint': '#ECEDF7',
    'text': '#1C1E33',
    'background': '#FFFFFF',
  },
  'electronics': {
    'primary': '#0A5BB5',
    'accent': '#0B7F65',
    'tint': '#E4EEFA',
    'text': '#14202E',
    'background': '#F4F7FB',
  },
};

void main() {
  group('contrastRatio (9.3 WCAG)', () {
    test('jana value', () {
      expect(contrastRatio('#000000', '#FFFFFF'), closeTo(21, 0.001));
      expect(contrastRatio('#FFFFFF', '#000000'), closeTo(21, 0.001));
      expect(contrastRatio('#777777', '#FFFFFF'), closeTo(4.48, 0.01));
      expect(contrastRatio('#1F5A32', '#1F5A32'), closeTo(1, 0.001));
    });
  });

  group('mixColor (9.3, v1.12 sRGB)', () {
    test('channel e linear, round', () {
      expect(mixColor('#000000', '#FFFFFF', 0.5), '#808080');
      expect(mixColor('#1F5A32', '#000000', 0.08), '#1D532E');
      expect(mixColor('#FBFBF6', '#FFFFFF', 0.5), '#FDFDFB');
    });
  });

  group('deriveTokens (9.3, 9.4, v1.12)', () {
    test('organic preset er token', () {
      final t = deriveTokens(presets['organic']!);
      expect(t, {
        'primary': '#1F5A32',
        'primary-hover': '#1D532E',
        'on-primary': '#FFFFFF',
        'accent': '#F2C230',
        'on-accent': '#3B2A1A',
        'tint': '#E3EED3',
        'text': '#3B2A1A',
        'text-muted': '#7C7165', // 65% e 4.5 na, tai text er dike shora
        'background': '#FBFBF6',
        'surface': '#FDFDFB',
        'border': '#E4E2DC',
      });
    });
    test('fashion, electronics: muted 65% e-i 4.5, on-accent shada', () {
      final f = deriveTokens(presets['fashion']!),
          e = deriveTokens(presets['electronics']!);
      expect(
        [f['text-muted'], f['border'], f['on-accent']],
        ['#6B6D7A', '#E4E4E7', '#FFFFFF'],
      );
      expect(
        [e['text-muted'], e['border'], e['on-accent']],
        ['#626B76', '#D9DDE2', '#FFFFFF'],
      );
    });
    for (final e in presets.entries) {
      test('${e.key}: muted background ar surface e ≥ 4.5, warning nai', () {
        final t = deriveTokens(e.value);
        expect(
          contrastRatio(t['text-muted']!, t['background']!),
          greaterThanOrEqualTo(4.5),
        );
        expect(
          contrastRatio(t['text-muted']!, t['surface']!),
          greaterThanOrEqualTo(4.5),
        );
        expect(t['text-muted'], isNot(t['text']));
        expect(contrastWarnings(t), isEmpty);
      });
    }
    test('bhul hex e ArgumentError', () {
      expect(
        () => deriveTokens({...presets['organic']!, 'text': 'red'}),
        throwsArgumentError,
      );
      expect(() => deriveTokens({'primary': '#000000'}), throwsArgumentError);
    });
  });

  group('contrastWarnings (9.3, v1.12)', () {
    test('kharap jora dhore', () {
      final t = deriveTokens({
        ...presets['fashion']!,
        'text': '#BBBBBB',
        'primary': '#F0F0F0',
      });
      final pairs = contrastWarnings(
        t,
      ).map((w) => '${w.foreground}/${w.background}').toSet();
      expect(
        pairs,
        containsAll([
          'text/background',
          'text/surface',
          'text/tint',
          'primary/background',
        ]),
      );
      expect(contrastWarnings(t).every((w) => w.ratio < 4.5), isTrue);
    });
  });
}
