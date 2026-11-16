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

// 9.3 v1.38 er notun jora: badge lekha/soft 4.5, `primary`/`primary-soft` 4.5, `focus` bg/surface 3.0.
void _newPairsOk(Map<String, String> t, {bool primarySoft = true}) {
  for (final k in ['success', 'danger', 'warning']) {
    expect(
      contrastRatio(t[k]!, t['$k-soft']!),
      greaterThanOrEqualTo(4.5),
      reason: k,
    );
    expect(
      contrastRatio(t[k]!, t['background']!),
      greaterThanOrEqualTo(4.5),
      reason: k,
    );
  }
  expect(
    contrastRatio(t['text']!, t['accent-soft']!),
    greaterThanOrEqualTo(4.5),
  );
  if (primarySoft) {
    expect(
      contrastRatio(t['primary']!, t['primary-soft']!),
      greaterThanOrEqualTo(4.5),
    );
  }
  for (final b in ['background', 'surface']) {
    expect(
      contrastRatio(t['focus']!, t[b]!),
      greaterThanOrEqualTo(3.0),
      reason: b,
    );
  }
}

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
        // v1.38 (MU): derived
        'primary-soft': '#E5EBE2',
        'accent-soft': '#F9F1D2',
        'success': '#1E6B3A',
        'success-soft': '#E0EADF',
        'danger': '#B42318',
        'danger-soft': '#F2E1DB',
        'warning': '#8A5A00',
        'warning-soft': '#EDE8D8',
        'focus': '#1F5A32',
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
    for (final e in presets.entries) {
      test(
        '${e.key}: notun jora (v1.38) badge/soft 4.5, primary-soft 4.5, focus 3.0',
        () {
          final t = deriveTokens(e.value);
          _newPairsOk(t);
          expect(t['focus'], t['primary']);
        },
      );
    }
    test('halka primary: focus text e jay, baki jora thik thake (v1.38)', () {
      final t = deriveTokens({...presets['organic']!, 'primary': '#A8D5B5'});
      expect(t['focus'], t['text']);
      expect(t['primary-soft'], t['background']);
      _newPairsOk(t, primarySoft: false);
    });
    test('rongin background e status rong text er dike shore (v1.38)', () {
      final t = deriveTokens({...presets['organic']!, 'background': '#C9A5A5'});
      expect(t['danger'], isNot('#B42318'));
      _newPairsOk(t, primarySoft: false);
    });
    test('bhul hex e ArgumentError', () {
      expect(
        () => deriveTokens({...presets['organic']!, 'text': 'red'}),
        throwsArgumentError,
      );
      expect(() => deriveTokens({'primary': '#000000'}), throwsArgumentError);
    });
  });

  group('cssVariables (9.3)', () {
    test('9.3 er naam, radius, font', () {
      final v = cssVariables(
        deriveTokens(presets['organic']!),
        radius: 14,
        headingFont: 'Baloo Da 2',
        bodyFont: 'Hind Siliguri',
      );
      expect(v['--color-bg'], '#FBFBF6');
      expect(v['--color-text-muted'], '#7C7165');
      expect(
        [v['--radius'], v['--font-heading'], v['--font-body']],
        ['14px', '"Baloo Da 2"', '"Hind Siliguri"'],
      );
      expect(v.length, 28);
    });
    test(
      'v1.38 notun variable: soft, status, focus, text-rgb, radius, shadow',
      () {
        final v = cssVariables(
          deriveTokens(presets['organic']!),
          radius: 14,
          headingFont: 'Baloo Da 2',
          bodyFont: 'Hind Siliguri',
        );
        expect(v, containsPair('--color-primary-soft', '#E5EBE2'));
        expect(v, containsPair('--color-accent-soft', '#F9F1D2'));
        for (final k in ['success', 'danger', 'warning']) {
          expect(v['--color-$k'], matches(r'^#[0-9A-F]{6}$'));
          expect(v['--color-$k-soft'], matches(r'^#[0-9A-F]{6}$'));
        }
        expect(v, containsPair('--color-focus', '#1F5A32'));
        expect(v, containsPair('--color-text-rgb', '59 42 26'));
        expect(v, containsPair('--radius-sm', '7px'));
        expect(v, containsPair('--radius-lg', '21px'));
        expect(v['--shadow-1'], contains('rgb(59 42 26 /'));
        expect(v['--shadow-2'], contains('rgb(59 42 26 /'));
        expect(v['--shadow-1'], isNot(v['--shadow-2']));
      },
    );
    test('radius-sm round(r/2), radius-lg min(round(r × 1.5), 24)', () {
      final t = deriveTokens(presets['organic']!);
      String r(int px, String k) =>
          cssVariables(t, radius: px, headingFont: 'A', bodyFont: 'A')[k]!;
      expect([r(2, '--radius-sm'), r(2, '--radius-lg')], ['1px', '3px']);
      expect([r(5, '--radius-sm'), r(5, '--radius-lg')], ['3px', '8px']);
      expect([r(0, '--radius-sm'), r(0, '--radius-lg')], ['0px', '0px']);
      expect(r(20, '--radius-lg'), '24px');
    });
    test('font e CSS dhukano ba bhul radius e error', () {
      final t = deriveTokens(presets['organic']!);
      expect(
        () => cssVariables(
          t,
          radius: 14,
          headingFont: 'x"; } body { display:none',
          bodyFont: 'A',
        ),
        throwsArgumentError,
      );
      expect(
        () => cssVariables(t, radius: -1, headingFont: 'A', bodyFont: 'A'),
        throwsArgumentError,
      );
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
