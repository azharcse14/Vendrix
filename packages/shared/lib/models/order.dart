import 'json.dart';
import 'localized_text.dart';

/// Delivery zone er district (`GET /api/delivery-zones`, 7.2, 8.19 item 2).
class DeliveryZone {
  /// District.
  final String district;

  /// Area er zone (locale e naam).
  final List<DeliveryArea> areas;

  /// District-wide zone ache (form e "Onno area").
  final bool districtWide;

  /// "Onno area" te delivery er din (server er 8.15 niyom); district-wide na hole null.
  final int? districtWideDays;

  /// DeliveryZone.
  const DeliveryZone(
    this.district,
    this.areas,
    this.districtWide,
    this.districtWideDays,
  );

  /// JSON theke.
  factory DeliveryZone.fromJson(Map<String, dynamic> j) => DeliveryZone(
    field(j, 'district'),
    objects(j, 'areas', DeliveryArea.fromJson),
    field(j, 'district_wide'),
    field(j, 'district_wide_days'),
  );
}

/// Zone er ek area: [area] request e jay, [name] dekhano.
class DeliveryArea {
  /// Area, naam.
  final String area, name;

  /// Delivery er din (server er 8.15 niyom, client hishab kore na).
  final int days;

  /// DeliveryArea.
  const DeliveryArea(this.area, this.name, this.days);

  /// JSON theke.
  factory DeliveryArea.fromJson(Map<String, dynamic> j) =>
      DeliveryArea(field(j, 'area'), field(j, 'name'), field(j, 'days'));
}

/// `POST /api/checkout/quote` (7.5, 8.5); taka poisha, shudhu server er (Rule 1).
class Quote {
  /// Taka.
  final int subtotal, discount, deliveryCharge, total;

  /// Order e OTP lagbe (8.7).
  final bool otpRequired;

  /// Order atkabe emon karon (`error.<code>`: `coupon_invalid`, `below_min_order` ...).
  final List<String> problems;

  /// Quote.
  const Quote(
    this.subtotal,
    this.discount,
    this.deliveryCharge,
    this.total,
    this.otpRequired,
    this.problems,
  );

  /// JSON theke.
  factory Quote.fromJson(Map<String, dynamic> j) => Quote(
    field(j, 'subtotal'),
    field(j, 'discount'),
    field(j, 'delivery_charge'),
    field(j, 'total'),
    field(j, 'otp_required'),
    (field<List<dynamic>?>(j, 'problems') ?? const []).cast<String>(),
  );
}

/// `POST /api/orders` er uttor (7.5, 8.6).
class PlacedOrder {
  /// Order number, page er token (Rule 11), status.
  final String orderNumber, publicToken, status;

  /// Online payment er URL (COD e null).
  final String? paymentUrl;

  /// PlacedOrder.
  const PlacedOrder(
    this.orderNumber,
    this.publicToken,
    this.status,
    this.paymentUrl,
  );

  /// JSON theke.
  factory PlacedOrder.fromJson(Map<String, dynamic> j) => PlacedOrder(
    field(j, 'order_number'),
    field(j, 'public_token'),
    field(j, 'status'),
    field(j, 'payment_url'),
  );
}

/// Order er ek line (snapshot, `{bn, en}`).
class OrderItem {
  /// Product er naam, variant er lekha (khali hote pare).
  final LocalizedText name, variantLabel;

  /// Quantity, line total.
  final int quantity, lineTotal;

  /// OrderItem.
  const OrderItem(this.name, this.variantLabel, this.quantity, this.lineTotal);

  /// JSON theke.
  factory OrderItem.fromJson(Map<String, dynamic> j) => OrderItem(
    LocalizedText.fromJson(field(j, 'name')),
    LocalizedText.fromJson(field(j, 'variant_label')),
    field(j, 'quantity'),
    field(j, 'line_total'),
  );
}

/// `GET /api/confirm/{confirm_token}` (8.10, 8.19 item 6): mask kora; thikana shudhu district + area.
class ConfirmSummary {
  /// Order number, `display_status`, customer naam, phone mask, district, area, payment method.
  final String orderNumber,
      displayStatus,
      customerName,
      phoneMasked,
      district,
      area,
      paymentMethod;

  /// Kokhon.
  final DateTime placedAt;

  /// Total (poisha).
  final int total;

  /// Active line.
  final List<OrderItem> items;

  /// JSON theke ([Order] o eta diye).
  ConfirmSummary.fromJson(Map<String, dynamic> j)
    : orderNumber = field(j, 'order_number'),
      displayStatus = field(j, 'display_status'),
      placedAt = time(j, 'placed_at'),
      customerName = field(j, 'customer_name'),
      phoneMasked = field(j, 'phone_masked'),
      district = field(j, 'district'),
      area = field(j, 'area'),
      paymentMethod = field(j, 'payment_method'),
      total = field(j, 'total'),
      items = objects(j, 'items', OrderItem.fromJson);
}

/// `GET /api/orders/{token}` (7.5, 8.19 item 6): [ConfirmSummary] + recipient ar taka.
class Order extends ConfirmSummary {
  /// Recipient er naam, phone mask.
  final String recipientName, recipientPhoneMasked;

  /// Taka.
  final int subtotal, discount, deliveryCharge;

  /// Delivery er shesh shomoy (na thakle null).
  final DateTime? deliveryDeadline;

  /// Customer ekhon cancel korte pare.
  final bool canCancel;

  /// JSON theke.
  Order.fromJson(super.j)
    : recipientName = field(j, 'recipient_name'),
      recipientPhoneMasked = field(j, 'recipient_phone_masked'),
      subtotal = field(j, 'subtotal'),
      discount = field(j, 'discount'),
      deliveryCharge = field(j, 'delivery_charge'),
      deliveryDeadline = timeOrNull(j, 'delivery_deadline'),
      canCancel = field(j, 'can_cancel'),
      super.fromJson();
}

/// `POST /api/orders/track` (7.5): token chhara, shudhu status ar itihash.
class TrackedOrder {
  /// Order number, `display_status`.
  final String orderNumber, displayStatus;

  /// Kokhon.
  final DateTime placedAt;

  /// Order level er status bodol.
  final List<OrderEvent> history;

  /// TrackedOrder.
  const TrackedOrder(
    this.orderNumber,
    this.displayStatus,
    this.placedAt,
    this.history,
  );

  /// JSON theke (`tracking` courier M7 e).
  factory TrackedOrder.fromJson(Map<String, dynamic> j) => TrackedOrder(
    field(j, 'order_number'),
    field(j, 'display_status'),
    time(j, 'placed_at'),
    objects(j, 'history', OrderEvent.fromJson),
  );
}

/// Itihash er ek ghotona.
class OrderEvent {
  /// Notun status.
  final String status;

  /// Kokhon.
  final DateTime at;

  /// OrderEvent.
  const OrderEvent(this.status, this.at);

  /// JSON theke.
  factory OrderEvent.fromJson(Map<String, dynamic> j) =>
      OrderEvent(field(j, 'status'), time(j, 'at'));
}

/// `GET /api/me/orders` er card (7.3, 8.19 item 12 x).
class OrderSummary {
  /// Order number, page er token, `display_status`.
  final String orderNumber, publicToken, displayStatus;

  /// Total (poisha).
  final int total;

  /// Kokhon.
  final DateTime placedAt;

  /// OrderSummary.
  const OrderSummary(
    this.orderNumber,
    this.publicToken,
    this.displayStatus,
    this.total,
    this.placedAt,
  );

  /// JSON theke.
  factory OrderSummary.fromJson(Map<String, dynamic> j) => OrderSummary(
    field(j, 'order_number'),
    field(j, 'public_token'),
    field(j, 'display_status'),
    field(j, 'total'),
    time(j, 'placed_at'),
  );
}

/// Status badge er rong (10.5 v1.38). Lekha shob shomoy thake; rong shudhu shathe (rong only na).
enum BadgeTone {
  /// Shesh, bhalo (delivered).
  success,

  /// Shesh, bad (cancelled).
  danger,

  /// Customer/shop er kaj baki (pending, partial).
  warning,

  /// Cholche.
  accent,

  /// Baki/ochena.
  muted,
}

/// `display_status`/itihash er status (8.8) → [BadgeTone]; `order.status.*` er protiti key (test dhore).
const orderStatusTones = <String, BadgeTone>{
  'pending': BadgeTone.warning,
  'confirmed': BadgeTone.accent,
  'open': BadgeTone.accent,
  'packed': BadgeTone.accent,
  'shipped': BadgeTone.accent,
  'partial': BadgeTone.warning,
  'delivered': BadgeTone.success,
  'returned': BadgeTone.muted,
  'cancelled': BadgeTone.danger,
};

/// [orderStatusTones]; ochena status (notun Go status) [BadgeTone.muted].
BadgeTone orderStatusTone(String status) =>
    orderStatusTones[status] ?? BadgeTone.muted;
