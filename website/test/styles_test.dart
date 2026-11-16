import 'dart:io';

import 'package:test/test.dart';

// 10.5 v1.38: styles.css minify ≤ 30 KB, gzip ≤ 7 KB. Tailwind standalone CLI (scripts/tailwind.sh) asol output mape.
void main() {
  test('styles.css budget ar base (10.5, 10.7 v1.38)', () async {
    final run = await Process.run('bash', ['../scripts/tailwind.sh', '--minify']);
    expect(run.exitCode, 0, reason: '${run.stdout}\n${run.stderr}');
    final css = File('web/styles.css').readAsBytesSync();
    final gz = gzip.encode(css).length;
    printOnFailure('styles.css: ${css.length} B, gzip $gz B');
    expect(css.length, lessThanOrEqualTo(30 * 1024));
    expect(gz, lessThanOrEqualTo(7 * 1024));
    final s = String.fromCharCodes(css);
    // 10.7 v1.38 er base niyom (purge hoy na, tai protiti page e thake).
    for (final rule in [
      ':focus-visible{outline:2px solid var(--color-focus)',
      'accent-color:var(--color-primary)',
      'prefers-reduced-motion:reduce',
      // `hidden` attribute `.notice`/`.input` er display ke harabe (MU U8).
      '[hidden]{display:none!important}',
      // MU review: bhul field er border (JS +0, aria-invalid client dey); chip er bhetore focus.
      'aria-invalid=true]{',
      '.chip:has(:focus-visible){outline:2px solid var(--color-focus)',
    ]) {
      expect(s.contains(rule), isTrue, reason: 'styles.css e nai: $rule');
    }
  }, timeout: const Timeout(Duration(minutes: 2)));
}
