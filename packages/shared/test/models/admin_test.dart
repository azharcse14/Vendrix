import 'package:shared/shared.dart';
import 'package:test/test.dart';

// Go er shape (M7a step 2–12 "Hoyeche"): order/store.go, admin.go, shipment.go, picklist.go, dashboard.go;
// inventory/store.go; auth/staff_admin.go, staff_store.go; audit/handler.go; customer/store.go;
// shipping/admin.go, coupon/admin.go (`to_jsonb` row); catalog/image.go.
final itemJson = <String, dynamic>{
  'id': 11,
  'fulfillment_id': 5,
  'variant_id': 3,
  'product_name': {'bn': 'লাল চাল', 'en': 'Red Rice'},
  'variant_label': {'bn': '৫ কেজি', 'en': '5 kg'},
  'sku': 'RICE-5',
  'unit_price': 45000,
  'quantity': 2,
  'variable_weight': false,
  'ordered_weight_g': null,
  'packed_weight_g': null,
  'line_total': 90000,
  'discount_share': 0,
  'status': 'active',
};

final orderJson = <String, dynamic>{
  'id': 7,
  'order_number': 'SB-100001',
  'status': 'confirmed',
  'display_status': 'confirmed',
  'recipient_name': 'রহিম',
  'recipient_phone': '+8801712345678',
  'district': 'ঢাকা',
  'area': 'ধানমন্ডি',
  'total': 96000,
  'placed_at': '2026-11-16T10:30:00.123456+06:00',
  'delivery_deadline': null,
  'customer_name': 'রহিম',
  'customer_phone': '+8801712345678',
  'payment_method': 'cod',
  'payment_status': 'unpaid',
  'risk_flags': ['cod_high_amount'],
  'subtotal': 90000,
  'discount': 0,
  'delivery_charge': 6000,
};

void main() {
  group('Admin model (7.1, 7.7, 11.7, M7a step 13)', () {
    test('AdminOrder: shop view; timestamp UTC (+06:00 o)', () {
      final o = AdminOrder.fromJson(orderJson);
      expect(o.placedAt, DateTime.utc(2026, 11, 16, 4, 30, 0, 123, 456));
      expect(o.placedAt.isUtc, isTrue);
      expect(
        [o.id, o.total, o.subtotal, o.deliveryCharge],
        [7, 96000, 90000, 6000],
      );
      expect(o.riskFlags, ['cod_high_amount']);
      expect(o.deliveryDeadline, isNull);
      expect(o.paymentMethod, 'cod');
    });
    test('AdminOrder: vendor role e payment/customer field nai (null)', () {
      final o = AdminOrder.fromJson(
        Map.of(orderJson)..removeWhere(
          (k, _) => const [
            'customer_name',
            'customer_phone',
            'payment_method',
            'payment_status',
            'risk_flags',
            'subtotal',
            'discount',
            'delivery_charge',
          ].contains(k),
        ),
      );
      expect([
        o.customerName,
        o.customerPhone,
        o.paymentMethod,
        o.paymentStatus,
        o.riskFlags,
        o.subtotal,
      ], everyElement(isNull));
    });
    test('AdminOrder: bhul type e FormatException', () {
      expect(
        () => AdminOrder.fromJson({...orderJson, 'total': '96000'}),
        throwsFormatException,
      );
    });
    test(
      'OrderDetail: fulfillment, item, history, note (vendor e note nai)',
      () {
        final d = OrderDetail.fromJson({
          ...orderJson,
          'delivery_deadline': '2026-11-21T18:00:00Z',
          'address': 'বাড়ি ১২',
          'note': null,
          'cancel_reason': null,
          'fulfillments': [
            {
              'id': 5,
              'vendor_id': 1,
              'seq': 1,
              'status': 'open',
              'subtotal': 90000,
              'discount': 0,
              'delivery_charge': 6000,
              'total': 96000,
              'items': [
                itemJson,
                {
                  ...itemJson,
                  'id': 12,
                  'variable_weight': true,
                  'ordered_weight_g': 1000,
                  'packed_weight_g': 1040,
                },
              ],
            },
          ],
          'history': [
            {
              'fulfillment_id': null,
              'from_status': null,
              'to_status': 'pending',
              'actor_type': 'customer',
              'actor_id': null,
              'note': null,
              'at': '2026-11-16T04:30:00Z',
            },
          ],
          'notes': [
            {
              'id': 1,
              'staff_id': 2,
              'kind': 'call',
              'note': 'ফোন ধরেনি',
              'created_at': '2026-11-16T05:00:00Z',
            },
          ],
        });
        expect(d.deliveryDeadline, DateTime.utc(2026, 11, 21, 18));
        expect(d.address, 'বাড়ি ১২');
        final f = d.fulfillments.single;
        expect(
          [f.id, f.vendorId, f.seq, f.status, f.total],
          [5, 1, 1, 'open', 96000],
        );
        expect(f.items.first.productName.of('en'), 'Red Rice');
        expect(f.items.first.orderedWeightG, isNull);
        expect(
          [f.items.last.orderedWeightG, f.items.last.packedWeightG],
          [1000, 1040],
        );
        expect(d.history.single.fromStatus, isNull);
        expect(d.history.single.toStatus, 'pending');
        expect(d.notes.single.kind, 'call');
        final vendor = OrderDetail.fromJson({
          ...orderJson,
          'address': 'x',
          'note': 'gate',
          'cancel_reason': null,
          'fulfillments': <Object>[],
          'history': <Object>[],
        });
        expect(vendor.notes, isEmpty);
        expect(vendor.note, 'gate');
      },
    );
    test('FulfillmentRow, Shipment (PATCH e kom field)', () {
      final f = FulfillmentRow.fromJson({
        'id': 5,
        'order_id': 7,
        'order_number': 'SB-100001',
        'vendor_id': 1,
        'seq': 1,
        'status': 'packed',
        'total': 96000,
        'recipient_name': 'রহিম',
        'district': 'ঢাকা',
        'area': 'ধানমন্ডি',
        'placed_at': '2026-11-16T04:30:00Z',
      });
      expect([f.orderId, f.orderNumber, f.status], [7, 'SB-100001', 'packed']);
      final s = Shipment.fromJson({
        'id': 9,
        'fulfillment_id': 5,
        'carrier': 'steadfast',
        'tracking_code': 'SF123',
        'status': 'booked',
        'cod_amount': 96000,
        'display_status': 'shipped',
      });
      expect(
        [s.id, s.fulfillmentId, s.carrier, s.trackingCode, s.codAmount],
        [9, 5, 'steadfast', 'SF123', 96000],
      );
      final p = Shipment.fromJson({
        'id': 9,
        'status': 'delivered',
        'display_status': 'delivered',
      });
      expect([
        p.fulfillmentId,
        p.carrier,
        p.trackingCode,
        p.codAmount,
      ], everyElement(isNull));
    });
    test('PickList', () {
      final v = {
        'variant_id': 3,
        'product_name': {'bn': 'মধু'},
        'variant_label': <String, dynamic>{},
        'sku': 'HONEY',
        'variable_weight': true,
        'quantity': 2,
        'ordered_weight_g': 1500,
      };
      final p = PickList.fromJson({
        'date': '2026-11-17',
        'variants': [v],
        'areas': [
          {
            'district': 'ঢাকা',
            'area': 'ধানমন্ডি',
            'variants': [v],
          },
        ],
        'fulfillments': [
          {
            'id': 5,
            'order_id': 7,
            'order_number': 'SB-100001',
            'seq': 1,
            'vendor_id': 1,
            'recipient_name': 'রহিম',
            'district': 'ঢাকা',
            'area': 'ধানমন্ডি',
            'confirmed_at': '2026-11-16T05:00:00Z',
          },
        ],
      });
      expect(p.date, '2026-11-17');
      expect(p.variants.single.orderedWeightG, 1500);
      expect(p.areas.single.variants.single.productName.of('en'), 'মধু');
      expect(p.fulfillments.single.confirmedAt, DateTime.utc(2026, 11, 16, 5));
    });
    test(
      'InventoryVariant (options er label), Batch, Movement (cost_price na thakte pare)',
      () {
        final v = InventoryVariant.fromJson({
          'variant_id': 3,
          'product_id': 2,
          'product_name': {'bn': 'লাল চাল', 'en': 'Red Rice'},
          'variant_label': {
            'weight': {'bn': '৫ কেজি', 'en': '5 kg'},
          },
          'sku': 'RICE-5',
          'stock': 4,
          'low_stock_at': 5,
          'batch_remaining': 4,
          'next_expiry': '2026-12-01',
        });
        expect(v.variantLabel['weight']!.of('en'), '5 kg');
        expect(
          [v.stock, v.lowStockAt, v.batchRemaining, v.nextExpiry],
          [4, 5, 4, '2026-12-01'],
        );
        final batch = {
          'id': 1,
          'variant_id': 3,
          'batch_code': null,
          'quantity_received': 10,
          'quantity_remaining': 4,
          'cost_price': 30000,
          'harvest_date': null,
          'expiry_date': '2026-12-01',
          'received_at': '2026-11-10T04:00:00Z',
        };
        expect(Batch.fromJson(batch).costPrice, 30000);
        final staffView = Batch.fromJson(Map.of(batch)..remove('cost_price'));
        expect(
          [staffView.costPrice, staffView.batchCode, staffView.expiryDate],
          [null, null, '2026-12-01'],
        );
        final m = Movement.fromJson({
          'id': 1,
          'variant_id': 3,
          'batch_id': 1,
          'change': -2,
          'reason': 'wastage',
          'order_id': null,
          'staff_id': 4,
          'note': 'পচে গেছে',
          'created_at': '2026-11-16T04:00:00Z',
        });
        expect(
          [m.change, m.reason, m.orderId, m.costPrice],
          [-2, 'wastage', null, null],
        );
      },
    );
    test('Coupon, Zone (to_jsonb row)', () {
      final c = Coupon.fromJson({
        'id': 1,
        'vendor_id': null,
        'funded_by': 'shop',
        'code': 'EID10',
        'kind': 'percent',
        'value': 10,
        'min_order': 50000,
        'max_discount': null,
        'usage_limit': 100,
        'used_count': 3,
        'per_phone_limit': 1,
        'starts_at': null,
        'expires_at': '2026-12-31T17:59:59+00:00',
        'is_active': true,
        'created_at': '2026-11-16T10:00:00+06:00',
      });
      expect(
        [c.code, c.kind, c.value, c.maxDiscount, c.usageLimit, c.usedCount],
        ['EID10', 'percent', 10, null, 100, 3],
      );
      expect(c.expiresAt, DateTime.utc(2026, 12, 31, 17, 59, 59));
      expect(c.createdAt, DateTime.utc(2026, 11, 16, 4));
      expect(c.startsAt, isNull);
      final z = Zone.fromJson({
        'id': 2,
        'name': {'bn': 'ঢাকা', 'en': 'Dhaka'},
        'district': 'ঢাকা',
        'area': null,
        'charge': 6000,
        'min_order': 0,
        'free_delivery_over': 200000,
        'same_city': true,
        'same_day_cutoff': null,
        'own_rider': false,
        'is_active': true,
      });
      expect(
        [z.area, z.charge, z.freeDeliveryOver, z.sameCity, z.isActive],
        [null, 6000, 200000, true, true],
      );
      expect(z.name.of('en'), 'Dhaka');
    });
    test('Customer: list row (count) ar detail (order)', () {
      final row = Customer.fromJson({
        'id': 1,
        'phone': '+8801712345678',
        'name': null,
        'email': null,
        'locale': 'bn',
        'created_at': '2026-11-01T00:00:00Z',
        'order_count': 3,
        'order_total': 150000,
        'blocked': false,
      });
      expect(
        [row.orderCount, row.orderTotal, row.name, row.orders],
        [3, 150000, null, isEmpty],
      );
      final d = Customer.fromJson({
        'id': 1,
        'phone': '+8801712345678',
        'name': 'রহিম',
        'email': 'r@example.com',
        'locale': 'en',
        'created_at': '2026-11-01T00:00:00Z',
        'blocked': true,
        'orders': [
          {
            'id': 7,
            'order_number': 'SB-100001',
            'display_status': 'delivered',
            'payment_method': 'cod',
            'payment_status': 'paid',
            'total': 96000,
            'placed_at': '2026-11-16T04:30:00Z',
          },
        ],
      });
      expect(
        [d.orderCount, d.blocked, d.orders.single.total],
        [null, true, 96000],
      );
      final b = BlockedPhone.fromJson({
        'phone': '+8801712345678',
        'reason': null,
        'staff_id': 1,
        'created_at': '2026-11-16T04:30:00Z',
      });
      expect([b.phone, b.reason, b.staffId], ['+8801712345678', null, 1]);
    });
    test('Staff (temp_password shudhu create e), StaffProfile', () {
      final row = {
        'id': 2,
        'email': 'staff@example.com',
        'name': 'করিম',
        'role': 'staff',
        'vendor_id': null,
        'is_active': true,
        'must_change_password': true,
        'totp_enabled': false,
        'locked_until': null,
        'last_login_at': null,
        'created_at': '2026-11-16T04:30:00Z',
      };
      final s = Staff.fromJson(row);
      expect(
        [
          s.role,
          s.mustChangePassword,
          s.totpEnabled,
          s.lastLoginAt,
          s.tempPassword,
        ],
        ['staff', true, false, null, null],
      );
      expect(
        Staff.fromJson({
          ...row,
          'temp_password': 'ABCDEFGHIJKLMNOPQRSTUVWXYZ',
        }).tempPassword,
        hasLength(26),
      );
      final me = StaffProfile.fromJson({
        'name': 'মালিক',
        'email': 'o@example.com',
        'role': 'owner',
        'vendor_id': null,
      });
      expect([me.role, me.vendorId], ['owner', null]);
    });
    test('AuditEntry (CLI row e staff nai), Dashboard, AdminImage', () {
      final a = AuditEntry.fromJson({
        'id': 1,
        'staff_id': null,
        'staff_name': null,
        'action': 'staff.reset_2fa',
        'entity_type': 'staff',
        'entity_id': 2,
        'before': null,
        'after': {'totp_cleared': true},
        'ip': null,
        'created_at': '2026-11-16T04:30:00Z',
      });
      expect(
        [a.staffId, a.staffName, a.before, a.after],
        [
          null,
          null,
          null,
          {'totp_cleared': true},
        ],
      );
      final d = Dashboard.fromJson({
        'date': '2026-11-17',
        'orders': 4,
        'sales': 200000,
        'pending': 1,
        'to_pack': 2,
        'low_stock': 3,
        'expiring': 0,
        'deadline_soon': 1,
      });
      expect(
        [
          d.date,
          d.orders,
          d.sales,
          d.pending,
          d.toPack,
          d.lowStock,
          d.expiring,
          d.deadlineSoon,
        ],
        ['2026-11-17', 4, 200000, 1, 2, 3, 0, 1],
      );
      final i = AdminImage.fromJson({
        'id': 1,
        'product_id': 2,
        'storage_key': 'shops/1/products/2/ABC.jpg',
        'alt': {'bn': 'চাল'},
        'width': 800,
        'height': 600,
        'sort_order': 0,
      });
      expect(
        [i.productId, i.storageKey, i.alt.of('en'), i.sortOrder],
        [2, 'shops/1/products/2/ABC.jpg', 'চাল', 0],
      );
    });
  });
}
