import 'package:shared/models/json.dart';
import 'package:shared/shared.dart';

// Print data (11.7 item 8, `GET /api/print`): shudhu website e lage (admin shudhu link khole), tai shared e na.

Map<String, dynamic> _obj(Map<String, dynamic> j, String key) =>
    j[key] is Map<String, dynamic> ? j[key] as Map<String, dynamic> : const {};

/// Slip/invoice er line (snapshot). Packing slip e dam null.
class PrintItem {
  /// Naam, variant.
  final LocalizedText productName, variantLabel;

  /// SKU.
  final String sku;

  /// Qty.
  final int quantity;

  /// Ojon (gram): packed thakle packed, na hole ordered; ojon-variable na hole null.
  final int? weightG;

  /// Ekok dam, line total (invoice e).
  final int? unitPrice, lineTotal;

  /// JSON theke.
  PrintItem.fromJson(Map<String, dynamic> j)
    : productName = LocalizedText.fromJson(_obj(j, 'product_name')),
      variantLabel = LocalizedText.fromJson(_obj(j, 'variant_label')),
      sku = field(j, 'sku'),
      quantity = field(j, 'quantity'),
      weightG = (j['packed_weight_g'] ?? j['ordered_weight_g']) as int?,
      unitPrice = j['unit_price'] as int?,
      lineTotal = j['line_total'] as int?;
}

/// Fulfillment er packing slip.
class Slip {
  /// Order number, recipient, thikana, payment.
  final String orderNumber, recipientName, recipientPhone, district, area, address, paymentMethod;

  /// Fulfillment er kram.
  final int seq;

  /// COD e courier er nite hobe (poisha), prepaid e 0.
  final int codAmount;

  /// Customer er note.
  final String? note;

  /// Line.
  final List<PrintItem> items;

  /// JSON theke.
  Slip.fromJson(Map<String, dynamic> j)
    : orderNumber = field(j, 'order_number'),
      seq = field(j, 'seq'),
      recipientName = field(j, 'recipient_name'),
      recipientPhone = field(j, 'recipient_phone'),
      district = field(j, 'district'),
      area = field(j, 'area'),
      address = field(j, 'address'),
      note = j['note'] as String?,
      paymentMethod = field(j, 'payment_method'),
      codAmount = field(j, 'cod_amount'),
      items = objects(j, 'items', PrintItem.fromJson);
}

/// Puro order er invoice.
class Invoice {
  /// Order number, customer, recipient, thikana, payment.
  final String orderNumber,
      customerName,
      customerPhone,
      recipientName,
      recipientPhone,
      district,
      area,
      address,
      paymentMethod,
      paymentStatus;

  /// Order er shomoy.
  final DateTime placedAt;

  /// Taka (snapshot, Rule 1).
  final int subtotal, discount, deliveryCharge, total;

  /// Line.
  final List<PrintItem> items;

  /// JSON theke.
  Invoice.fromJson(Map<String, dynamic> j)
    : orderNumber = field(j, 'order_number'),
      placedAt = time(j, 'placed_at'),
      customerName = field(j, 'customer_name'),
      customerPhone = field(j, 'customer_phone'),
      recipientName = field(j, 'recipient_name'),
      recipientPhone = field(j, 'recipient_phone'),
      district = field(j, 'district'),
      area = field(j, 'area'),
      address = field(j, 'address'),
      paymentMethod = field(j, 'payment_method'),
      paymentStatus = field(j, 'payment_status'),
      subtotal = field(j, 'subtotal'),
      discount = field(j, 'discount'),
      deliveryCharge = field(j, 'delivery_charge'),
      total = field(j, 'total'),
      items = objects(j, 'items', PrintItem.fromJson);
}

/// `GET /api/print` er uttor: kind onujayi ekta.
typedef PrintData = ({List<Slip> slips, List<Invoice> invoices, PickList? pickList});

/// `GET /api/print?kind=&token=` (public, SSR; shop `X-Shop-Host` theke). Bhul token Go te `404 not_found`.
Future<PrintData> loadPrint(ApiClient api, String kind, String token) async {
  final j = await api.get('/api/print', query: {'kind': kind, 'token': token}) as Map<String, dynamic>;
  return (
    slips: objects(j, 'slips', Slip.fromJson),
    invoices: objects(j, 'invoices', Invoice.fromJson),
    pickList: j['pick_list'] is Map<String, dynamic> ? PickList.fromJson(j['pick_list'] as Map<String, dynamic>) : null,
  );
}
