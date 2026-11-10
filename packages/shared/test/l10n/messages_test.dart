import 'dart:io';

import 'package:shared/shared.dart';
import 'package:test/test.dart';

void main() {
  group('Default lekha (9.8, 7.1)', () {
    test('bn ar en e ekoi key', () {
      expect(
        defaultMessages['en']!.keys.toSet(),
        defaultMessages['bn']!.keys.toSet(),
      );
      for (final m in defaultMessages.values) {
        expect(m.values.where((v) => v.trim().isEmpty), isEmpty);
      }
    });
    test(
      'Go er protiti error code (httpx/message.go) er error.<code> ache',
      () {
        final go = File(
          '../../backend/internal/httpx/message.go',
        ).readAsStringSync();
        final codes = RegExp(
          r'^\s*"([a-z_]+)":\s*\{',
          multiLine: true,
        ).allMatches(go).map((m) => m[1]!).toList();
        expect(codes, contains('not_found'));
        for (final c in [...codes, 'network_error', 'timeout']) {
          expect(defaultMessages['bn'], contains('error.$c'), reason: c);
        }
        expect(defaultMessages['bn'], contains('theme.low_contrast'));
      },
    );
  });
}
