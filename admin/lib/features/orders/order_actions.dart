import 'package:shared/shared.dart';

/// Order er kaj (detail er button).
enum OrderAction { confirm, cancel }

/// Fulfillment er kaj; shipped er porer kaj shipment screen e.
enum FulfillmentAction { pack, undo, ship, cancel }

const _shopRoles = {'owner', 'admin', 'staff'};
const _ownerAdmin = {'owner', 'admin'};

// Backend er CanTransitionOrder/CanTransitionFulfillment ar admin.go er role niyom (11.2, 11.7 13 i) er
// moto, shudhu button lukanor jonno; suraksha backend e. Vendor role = Actor Vendor.

/// [o] te [role] kon order kaj korte pare.
Set<OrderAction> orderActions(OrderDetail o, String role) {
  if (!_shopRoles.contains(role)) return {};
  final fs = o.fulfillments.map((f) => f.status);
  return {
    // online e shudhu paid hole (payment_required)
    if (o.status == 'pending' &&
        (o.paymentMethod == 'cod' || o.paymentStatus == 'paid'))
      OrderAction.confirm,
    if (_ownerAdmin.contains(role) &&
        (o.status == 'pending' ||
            (o.status == 'confirmed' &&
                fs.every(
                  (s) => s == 'open' || s == 'packed' || s == 'cancelled',
                ))))
      OrderAction.cancel,
  };
}

/// [o] er fulfillment [f] e [role] kon kaj korte pare.
Set<FulfillmentAction> fulfillmentActions(
  OrderDetail o,
  Fulfillment f,
  String role,
) => switch (f.status) {
  'open' => {
    if (o.status == 'confirmed') FulfillmentAction.pack,
    // staff ar vendor_staff fulfillment cancel na (11.2 v1.7)
    if (role != 'staff' && role != 'vendor_staff') FulfillmentAction.cancel,
  },
  'packed' => {
    // undo shudhu COD (8.8 v1.37); vendor role e method null, backend dekhe
    if (o.paymentMethod == null || o.paymentMethod == 'cod')
      FulfillmentAction.undo,
    FulfillmentAction.ship,
    // packed → cancelled shudhu Staff actor, ar staff role cancel na
    if (_ownerAdmin.contains(role)) FulfillmentAction.cancel,
  },
  _ => {},
};
