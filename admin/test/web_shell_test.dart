import 'dart:io';

import 'package:flutter_test/flutter_test.dart';

// Browser tab ar install er naam Flutter er default na (M7b Safari proman e paoa).
void main() {
  test('web/index.html ar manifest e admin er naam', () {
    final html = File('web/index.html').readAsStringSync();
    final manifest = File('web/manifest.json').readAsStringSync();
    expect(html, contains('<title>Vendrix Admin</title>'));
    for (final s in [html, manifest]) {
      expect(s, isNot(contains('A new Flutter project')));
      expect(s, isNot(contains('vendrix_admin')));
    }
  });
}
