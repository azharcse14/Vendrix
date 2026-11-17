import 'package:admin/features/orders/order_actions.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared/shared.dart';

import '../../helpers.dart';

OrderDetail detail({
  String status = 'pending',
  String method = 'cod',
  String payment = 'unpaid',
  List<String> fs = const ['open'],
}) => OrderDetail.fromJson(
  orderJson(
    1,
    status: status,
    method: method,
    payment: payment,
    fulfillments: [
      for (final (i, f) in fs.indexed) fulfillmentJson(10 + i, status: f),
    ],
  ),
);

void main() {
  const shop = ['owner', 'admin', 'staff'];
  const vendor = ['vendor_owner', 'vendor_staff'];

  group('orderActions (CanTransitionOrder, 11.2, 11.4)', () {
    final cases = <(String, OrderDetail, String, Set<OrderAction>)>[
      (
        'COD pending owner',
        detail(),
        'owner',
        {OrderAction.confirm, OrderAction.cancel},
      ),
      (
        'COD pending staff: cancel na',
        detail(),
        'staff',
        {OrderAction.confirm},
      ),
      // online unpaid e confirm nai (payment_required)
      (
        'online unpaid',
        detail(method: 'sslcommerz'),
        'owner',
        {OrderAction.cancel},
      ),
      (
        'online paid',
        detail(method: 'sslcommerz', payment: 'paid'),
        'admin',
        {OrderAction.confirm, OrderAction.cancel},
      ),
      (
        'confirmed open/packed/cancelled',
        detail(status: 'confirmed', fs: ['open', 'packed', 'cancelled']),
        'owner',
        {OrderAction.cancel},
      ),
      (
        'confirmed shipped: cancel na',
        detail(status: 'confirmed', fs: ['open', 'shipped']),
        'owner',
        {},
      ),
      (
        'cancelled',
        detail(status: 'cancelled', fs: ['cancelled']),
        'owner',
        {},
      ),
    ];
    for (final (name, o, role, want) in cases) {
      test(name, () => expect(orderActions(o, role), want));
    }
    test('vendor role e order er kaj nai', () {
      for (final r in vendor) {
        expect(orderActions(detail(), r), isEmpty);
      }
    });
  });

  group('fulfillmentActions (CanTransitionFulfillment, 11.2)', () {
    Set<FulfillmentAction> f(
      String fs,
      String role, {
      String status = 'confirmed',
      String method = 'cod',
    }) {
      final o = detail(
        status: status,
        method: method,
        payment: 'paid',
        fs: [fs],
      );
      return fulfillmentActions(o, o.fulfillments.first, role);
    }

    test('open: pack shobai; cancel owner/admin/vendor_owner', () {
      for (final r in [...shop, ...vendor]) {
        final want = {FulfillmentAction.pack};
        if (!{'staff', 'vendor_staff'}.contains(r)) {
          want.add(FulfillmentAction.cancel);
        }
        expect(f('open', r), want, reason: r);
      }
    });
    test('pending order e pack na', () {
      expect(f('open', 'owner', status: 'pending'), {FulfillmentAction.cancel});
    });
    test('packed COD: undo, ship; cancel shudhu owner/admin', () {
      expect(f('packed', 'owner'), {
        FulfillmentAction.undo,
        FulfillmentAction.ship,
        FulfillmentAction.cancel,
      });
      expect(f('packed', 'staff'), {
        FulfillmentAction.undo,
        FulfillmentAction.ship,
      });
      expect(f('packed', 'vendor_owner'), {
        FulfillmentAction.undo,
        FulfillmentAction.ship,
      });
    });
    test('packed prepaid: undo na', () {
      expect(f('packed', 'staff', method: 'sslcommerz'), {
        FulfillmentAction.ship,
      });
    });
    test(
      'shipped/delivered/cancelled: detail e kichu na (shipment screen e)',
      () {
        for (final s in ['shipped', 'delivered', 'cancelled', 'returned']) {
          expect(f(s, 'owner'), isEmpty, reason: s);
        }
      },
    );
  });

  group('canEdit (11.7 item 7: shudhu kom, open fulfillment)', () {
    test('pending/confirmed COD, open item: shop role', () {
      for (final r in shop) {
        expect(canEdit(detail(), r), isTrue, reason: r);
        expect(canEdit(detail(status: 'confirmed'), r), isTrue, reason: r);
      }
    });
    test('vendor role na', () {
      for (final r in vendor) {
        expect(canEdit(detail(), r), isFalse, reason: r);
      }
    });
    test('packed (age undo), shipped, cancelled order e na', () {
      expect(
        canEdit(detail(status: 'confirmed', fs: ['packed']), 'owner'),
        isFalse,
      );
      expect(
        canEdit(detail(status: 'cancelled', fs: ['cancelled']), 'owner'),
        isFalse,
      );
    });
    test('online unpaid e na (payment_required), paid e hya', () {
      expect(canEdit(detail(method: 'sslcommerz'), 'owner'), isFalse);
      expect(
        canEdit(detail(method: 'sslcommerz', payment: 'paid'), 'owner'),
        isTrue,
      );
    });
  });

  group('canReturn (11.7 item 6)', () {
    bool r(String fs, String role) {
      final o = detail(status: 'confirmed', fs: [fs]);
      return canReturn(o, o.fulfillments.first, role);
    }

    test('shipped/delivered: shop role (refund er 403 backend e)', () {
      for (final role in shop) {
        expect(r('shipped', role), isTrue, reason: role);
        expect(r('delivered', role), isTrue, reason: role);
      }
    });
    test('vendor role ar onno status e na', () {
      for (final role in vendor) {
        expect(r('delivered', role), isFalse, reason: role);
      }
      for (final s in ['open', 'packed', 'returned', 'cancelled']) {
        expect(r(s, 'owner'), isFalse, reason: s);
      }
    });
  });
}
