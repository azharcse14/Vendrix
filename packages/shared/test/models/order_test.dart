import 'package:shared/shared.dart';
import 'package:test/test.dart';

// Go er shape (order/service.go, view.go, confirm.go; shipping/store.go; M6.md step 4–10)
final summaryJson = <String, dynamic>{
  'order_number': 'SB-100001',
  'display_status': 'pending',
  'placed_at': '2026-11-16T04:30:00.123456Z',
  'customer_name': 'রহিম',
  'phone_masked': '+88017****5678',
  'district': 'ঢাকা',
  'area': 'ধানমন্ডি',
  'payment_method': 'cod',
  'total': 96000,
  'items': [
    {
      'name': {'bn': 'লাল চাল', 'en': 'Red Rice'},
      'variant_label': {'bn': '৫ কেজি', 'en': '5 kg'},
      'quantity': 2,
      'line_total': 90000,
    },
    {
      'name': {'bn': 'মধু'},
      'variant_label': <String, dynamic>{},
      'quantity': 1,
      'line_total': 6000,
    },
  ],
};

void main() {
  group('Checkout ar order model (7.2, 7.3, 7.5, 8.19, M6)', () {
    test('Quote', () {
      final q = Quote.fromJson({
        'subtotal': 90000,
        'discount': 5000,
        'delivery_charge': 6000,
        'total': 91000,
        'otp_required': true,
        'problems': ['coupon_invalid', 'below_min_order'],
      });
      expect(
        [
          q.subtotal,
          q.discount,
          q.deliveryCharge,
          q.total,
          q.otpRequired,
          q.problems,
        ],
        [
          90000,
          5000,
          6000,
          91000,
          true,
          ['coupon_invalid', 'below_min_order'],
        ],
      );
      // Go nil slice → null
      expect(
        Quote.fromJson({
          'subtotal': 0,
          'discount': 0,
          'delivery_charge': 0,
          'total': 0,
          'otp_required': false,
          'problems': null,
        }).problems,
        isEmpty,
      );
    });
    test('PlacedOrder: payment_url null (COD)', () {
      final p = PlacedOrder.fromJson({
        'order_number': 'SB-100001',
        'public_token': 'TOKEN26',
        'status': 'pending',
        'payment_url': null,
      });
      expect(
        [p.orderNumber, p.publicToken, p.status, p.paymentUrl],
        ['SB-100001', 'TOKEN26', 'pending', null],
      );
    });
    test('ConfirmSummary: item er {bn, en}, khali variant label', () {
      final s = ConfirmSummary.fromJson(summaryJson);
      expect(
        [
          s.orderNumber,
          s.displayStatus,
          s.placedAt,
          s.customerName,
          s.phoneMasked,
          s.district,
          s.area,
          s.paymentMethod,
          s.total,
        ],
        [
          'SB-100001',
          'pending',
          DateTime.utc(2026, 11, 16, 4, 30, 0, 123, 456),
          'রহিম',
          '+88017****5678',
          'ঢাকা',
          'ধানমন্ডি',
          'cod',
          96000,
        ],
      );
      final a = s.items.first, b = s.items.last;
      expect(
        [a.name.of('en'), a.variantLabel.of('bn'), a.quantity, a.lineTotal],
        ['Red Rice', '৫ কেজি', 2, 90000],
      );
      expect([b.name.of('en'), b.variantLabel.of('en')], ['মধু', '']);
    });
    test('Order: summary + recipient, taka, deadline, can_cancel', () {
      final o = Order.fromJson({
        ...summaryJson,
        'recipient_name': 'করিম',
        'recipient_phone_masked': '+88018****1234',
        'subtotal': 90000,
        'discount': 0,
        'delivery_charge': 6000,
        'delivery_deadline': '2026-11-21T10:00:00+06:00',
        'can_cancel': true,
      });
      expect(
        [
          o.orderNumber,
          o.items.length,
          o.recipientName,
          o.recipientPhoneMasked,
          o.subtotal,
          o.discount,
          o.deliveryCharge,
          o.total,
          o.deliveryDeadline,
          o.canCancel,
        ],
        [
          'SB-100001',
          2,
          'করিম',
          '+88018****1234',
          90000,
          0,
          6000,
          96000,
          DateTime.utc(2026, 11, 21, 4),
          true,
        ],
      );
      expect(
        Order.fromJson({
          ...summaryJson,
          'recipient_name': 'রহিম',
          'recipient_phone_masked': '+88017****5678',
          'subtotal': 90000,
          'discount': 0,
          'delivery_charge': 6000,
          'delivery_deadline': null,
          'can_cancel': false,
        }).deliveryDeadline,
        isNull,
      );
    });
    test('TrackedOrder: history, tracking null', () {
      final t = TrackedOrder.fromJson({
        'order_number': 'SB-100001',
        'display_status': 'confirmed',
        'placed_at': '2026-11-16T04:30:00Z',
        'history': [
          {'status': 'pending', 'at': '2026-11-16T04:30:00Z'},
          {'status': 'confirmed', 'at': '2026-11-16T05:00:00Z'},
        ],
        'tracking': null,
      });
      expect(
        [t.orderNumber, t.displayStatus, t.placedAt, t.history.length],
        ['SB-100001', 'confirmed', DateTime.utc(2026, 11, 16, 4, 30), 2],
      );
      expect(
        [t.history.last.status, t.history.last.at],
        ['confirmed', DateTime.utc(2026, 11, 16, 5)],
      );
    });
    test('OrderSummary (me/orders card) Page e', () {
      final p = Page.fromJson({
        'items': [
          {
            'order_number': 'SB-100002',
            'public_token': 'TOK2',
            'display_status': 'cancelled',
            'total': 50000,
            'placed_at': '2026-11-16T06:00:00Z',
          },
        ],
        'page': 1,
        'limit': 20,
        'total': 1,
      }, OrderSummary.fromJson);
      final c = p.items.single;
      expect(
        [c.orderNumber, c.publicToken, c.displayStatus, c.total, c.placedAt],
        [
          'SB-100002',
          'TOK2',
          'cancelled',
          50000,
          DateTime.utc(2026, 11, 16, 6),
        ],
      );
    });
    test('DeliveryZone: district, area, din, district_wide (7.2 v1.35)', () {
      final z = DeliveryZone.fromJson({
        'district': 'ঢাকা',
        'areas': [
          {'area': 'dhanmondi', 'name': 'ধানমন্ডি', 'days': 5},
        ],
        'district_wide': true,
        'district_wide_days': 10,
      });
      expect(
        [
          z.district,
          z.areas.single.area,
          z.areas.single.name,
          z.areas.single.days,
          z.districtWide,
          z.districtWideDays,
        ],
        ['ঢাকা', 'dhanmondi', 'ধানমন্ডি', 5, true, 10],
      );
      final n = DeliveryZone.fromJson({
        'district': 'খুলনা',
        'areas': <Object>[],
        'district_wide': false,
        'district_wide_days': null,
      });
      expect(n.districtWideDays, isNull);
    });
    test('shape bhul hole FormatException', () {
      expect(
        () => OrderSummary.fromJson({
          'order_number': 'SB-1',
          'public_token': 'T',
          'display_status': 'pending',
          'total': '500',
          'placed_at': '2026-11-16T06:00:00Z',
        }),
        throwsFormatException,
      );
      expect(
        () => ConfirmSummary.fromJson({...summaryJson, 'placed_at': 'kal'}),
        throwsFormatException,
      );
    });
  });
}
