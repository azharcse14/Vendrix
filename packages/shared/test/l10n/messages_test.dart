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
    test(
      'Go er protiti cart notice (cart/service.go) er cart.notice.<code> ache',
      () {
        final go = File(
          '../../backend/internal/cart/service.go',
        ).readAsStringSync();
        final codes = RegExp(
          r'Notice\{Code: "([a-z_]+)"',
        ).allMatches(go).map((m) => m[1]!).toSet();
        expect(
          codes,
          containsAll(['item_removed', 'quantity_reduced', 'item_dropped']),
        );
        for (final c in codes) {
          expect(defaultMessages['bn'], contains('cart.notice.$c'), reason: c);
        }
      },
    );
    test(
      'Go er protiti order/fulfillment status (order/order.go) ar partial er order.status.<s> ache',
      () {
        final go = File(
          '../../backend/internal/order/order.go',
        ).readAsStringSync();
        final codes = RegExp(
          r'F?Status = "([a-z_]+)"',
        ).allMatches(go).map((m) => m[1]!).toSet();
        expect(codes, containsAll(['pending', 'confirmed', 'delivered']));
        for (final c in [...codes, 'partial']) {
          expect(defaultMessages['bn'], contains('order.status.$c'), reason: c);
        }
      },
    );
  });
}
