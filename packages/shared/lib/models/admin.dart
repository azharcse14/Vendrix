import 'json.dart';
import 'localized_text.dart';

// Admin API (7.7, 11.7) er response: shudhu Go je field dey; taka poisha `int`, din (`YYYY-MM-DD`, shop er
// timezone) `String`, shomoy UTC `DateTime`. Content `{bn, en}` [LocalizedText] (7.1).

Map<String, dynamic> _obj(Map<String, dynamic> j, String key) =>
    field<Map<String, dynamic>>(j, key);

/// `GET /api/admin/me` (7.7): role diye menu.
class StaffProfile {
  /// Naam, email, role (`owner`, `admin`, `staff`, `vendor_owner`, `vendor_staff`).
  final String name, email, role;

  /// Vendor role er vendor (shop staff e null).
  final int? vendorId;

  /// JSON theke.
  StaffProfile.fromJson(Map<String, dynamic> j)
    : name = field(j, 'name'),
      email = field(j, 'email'),
      role = field(j, 'role'),
      vendorId = field(j, 'vendor_id');
}

/// `/api/admin/staff` er row (11.7 item 11, v1.46).
class Staff {
  /// Id.
  final int id;

  /// Email, naam, role.
  final String email, name, role;

  /// Vendor role er vendor.
  final int? vendorId;

  /// Active, password bodlate hobe, TOTP chalu.
  final bool isActive, mustChangePassword, totpEnabled;

  /// Lock (ager hole null), shesh login.
  final DateTime? lockedUntil, lastLoginAt;

  /// Kokhon.
  final DateTime createdAt;

  /// Shudhu create er response e, ekbar (dekhiye bhule jao).
  final String? tempPassword;

  /// JSON theke.
  Staff.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      email = field(j, 'email'),
      name = field(j, 'name'),
      role = field(j, 'role'),
      vendorId = field(j, 'vendor_id'),
      isActive = field(j, 'is_active'),
      mustChangePassword = field(j, 'must_change_password'),
      totpEnabled = field(j, 'totp_enabled'),
      lockedUntil = timeOrNull(j, 'locked_until'),
      lastLoginAt = timeOrNull(j, 'last_login_at'),
      createdAt = time(j, 'created_at'),
      tempPassword = field(j, 'temp_password');
}

/// `GET /api/admin/orders` er row (11.7 item 13 vii). Vendor role e shop view er field null, [total] nijer ongsho.
class AdminOrder {
  /// Id.
  final int id;

  /// Order number, status, `display_status`, recipient, district, area.
  final String orderNumber,
      status,
      displayStatus,
      recipientName,
      recipientPhone,
      district,
      area;

  /// Total.
  final int total;

  /// Kokhon.
  final DateTime placedAt;

  /// Delivery er shesh shomoy.
  final DateTime? deliveryDeadline;

  /// Shudhu shop staff (vendor role e null).
  final String? customerName, customerPhone, paymentMethod, paymentStatus;

  /// Risk flag (8.7); vendor role e null.
  final List<String>? riskFlags;

  /// Taka; vendor role e null.
  final int? subtotal, discount, deliveryCharge;

  /// JSON theke.
  AdminOrder.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      orderNumber = field(j, 'order_number'),
      status = field(j, 'status'),
      displayStatus = field(j, 'display_status'),
      recipientName = field(j, 'recipient_name'),
      recipientPhone = field(j, 'recipient_phone'),
      district = field(j, 'district'),
      area = field(j, 'area'),
      total = field(j, 'total'),
      placedAt = time(j, 'placed_at'),
      deliveryDeadline = timeOrNull(j, 'delivery_deadline'),
      customerName = field(j, 'customer_name'),
      customerPhone = field(j, 'customer_phone'),
      paymentMethod = field(j, 'payment_method'),
      paymentStatus = field(j, 'payment_status'),
      riskFlags = field<List<dynamic>?>(j, 'risk_flags')?.cast<String>(),
      subtotal = field(j, 'subtotal'),
      discount = field(j, 'discount'),
      deliveryCharge = field(j, 'delivery_charge');
}

/// `GET /api/admin/orders/{id}`: vendor role e nijer fulfillment/history, [notes] khali.
class OrderDetail extends AdminOrder {
  /// Thikana.
  final String address;

  /// Customer er note, cancel er karon.
  final String? note, cancelReason;

  /// Fulfillment (item shoho).
  final List<Fulfillment> fulfillments;

  /// Status bodol.
  final List<HistoryEntry> history;

  /// Staff er note.
  final List<OrderNote> notes;

  /// JSON theke.
  OrderDetail.fromJson(super.j)
    : address = field(j, 'address'),
      note = field(j, 'note'),
      cancelReason = field(j, 'cancel_reason'),
      fulfillments = objects(j, 'fulfillments', Fulfillment.fromJson),
      history = objects(j, 'history', HistoryEntry.fromJson),
      notes = objects(j, 'notes', OrderNote.fromJson),
      super.fromJson();
}

/// Order detail er fulfillment (vendor er bhag).
class Fulfillment {
  /// Id, vendor, kram.
  final int id, vendorId, seq;

  /// Status (8.8).
  final String status;

  /// Taka.
  final int subtotal, discount, deliveryCharge, total;

  /// Line.
  final List<AdminItem> items;

  /// JSON theke.
  Fulfillment.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      vendorId = field(j, 'vendor_id'),
      seq = field(j, 'seq'),
      status = field(j, 'status'),
      subtotal = field(j, 'subtotal'),
      discount = field(j, 'discount'),
      deliveryCharge = field(j, 'delivery_charge'),
      total = field(j, 'total'),
      items = objects(j, 'items', AdminItem.fromJson);
}

/// Order er line (snapshot).
class AdminItem {
  /// Id, fulfillment, variant.
  final int id, fulfillmentId, variantId;

  /// Naam, variant er lekha.
  final LocalizedText productName, variantLabel;

  /// SKU, status (`active`, `removed` ...).
  final String sku, status;

  /// Dam, qty, line, discount er bhag.
  final int unitPrice, quantity, lineTotal, discountShare;

  /// Ojon-variable (pack e ojon lage).
  final bool variableWeight;

  /// Gram (ojon-variable na hole null).
  final int? orderedWeightG, packedWeightG;

  /// JSON theke.
  AdminItem.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      fulfillmentId = field(j, 'fulfillment_id'),
      variantId = field(j, 'variant_id'),
      productName = LocalizedText.fromJson(_obj(j, 'product_name')),
      variantLabel = LocalizedText.fromJson(_obj(j, 'variant_label')),
      sku = field(j, 'sku'),
      unitPrice = field(j, 'unit_price'),
      quantity = field(j, 'quantity'),
      variableWeight = field(j, 'variable_weight'),
      orderedWeightG = field(j, 'ordered_weight_g'),
      packedWeightG = field(j, 'packed_weight_g'),
      lineTotal = field(j, 'line_total'),
      discountShare = field(j, 'discount_share'),
      status = field(j, 'status');
}

/// Order/fulfillment er status bodol (order level e [fulfillmentId] null).
class HistoryEntry {
  /// Fulfillment, actor.
  final int? fulfillmentId, actorId;

  /// Ager status (shuru te null), note.
  final String? fromStatus, note;

  /// Notun status, actor type.
  final String toStatus, actorType;

  /// Kokhon.
  final DateTime at;

  /// JSON theke.
  HistoryEntry.fromJson(Map<String, dynamic> j)
    : fulfillmentId = field(j, 'fulfillment_id'),
      fromStatus = field(j, 'from_status'),
      toStatus = field(j, 'to_status'),
      actorType = field(j, 'actor_type'),
      actorId = field(j, 'actor_id'),
      note = field(j, 'note'),
      at = time(j, 'at');
}

/// Staff er note (`kind` `note` ba `call`).
class OrderNote {
  /// Id, staff.
  final int id, staffId;

  /// Kind, lekha.
  final String kind, note;

  /// Kokhon.
  final DateTime createdAt;

  /// JSON theke.
  OrderNote.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      staffId = field(j, 'staff_id'),
      kind = field(j, 'kind'),
      note = field(j, 'note'),
      createdAt = time(j, 'created_at');
}

/// `GET /api/admin/fulfillments` er row.
class FulfillmentRow {
  /// Id, order, vendor, kram, total.
  final int id, orderId, vendorId, seq, total;

  /// Order number, status, recipient, district, area.
  final String orderNumber, status, recipientName, district, area;

  /// Order kokhon.
  final DateTime placedAt;

  /// Active shipment (deliver/ferot er id; v1.50), na thakle null.
  final Shipment? shipment;

  /// JSON theke.
  FulfillmentRow.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      orderId = field(j, 'order_id'),
      orderNumber = field(j, 'order_number'),
      vendorId = field(j, 'vendor_id'),
      seq = field(j, 'seq'),
      status = field(j, 'status'),
      total = field(j, 'total'),
      recipientName = field(j, 'recipient_name'),
      district = field(j, 'district'),
      area = field(j, 'area'),
      placedAt = time(j, 'placed_at'),
      shipment = switch (field<Map<String, dynamic>?>(j, 'shipment')) {
        final m? => Shipment.fromJson(m),
        null => null,
      };
}

/// Shipment: POST/PATCH er uttor (PATCH e shudhu [id], [status], [displayStatus]) ba [FulfillmentRow.shipment]
/// ([displayStatus] chhara).
class Shipment {
  /// Id.
  final int id;

  /// Shipment status.
  final String status;

  /// Order er `display_status` (shudhu POST/PATCH e).
  final String? displayStatus;

  /// Shudhu create e.
  final int? fulfillmentId, codAmount;

  /// Shudhu create e.
  final String? carrier, trackingCode;

  /// JSON theke.
  Shipment.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      fulfillmentId = field(j, 'fulfillment_id'),
      carrier = field(j, 'carrier'),
      trackingCode = field(j, 'tracking_code'),
      status = field(j, 'status'),
      codAmount = field(j, 'cod_amount'),
      displayStatus = field(j, 'display_status');
}

/// `GET /api/admin/packing/pick-list` (11.7 item 4).
class PickList {
  /// Din (shop timezone).
  final String date;

  /// Variant onujayi mot.
  final List<PickVariant> variants;

  /// Area onujayi.
  final List<PickArea> areas;

  /// Fulfillment.
  final List<PickFulfillment> fulfillments;

  /// JSON theke.
  PickList.fromJson(Map<String, dynamic> j)
    : date = field(j, 'date'),
      variants = objects(j, 'variants', PickVariant.fromJson),
      areas = objects(j, 'areas', PickArea.fromJson),
      fulfillments = objects(j, 'fulfillments', PickFulfillment.fromJson);
}

/// Pick list er variant.
class PickVariant {
  /// Variant, qty.
  final int variantId, quantity;

  /// Naam, variant er lekha.
  final LocalizedText productName, variantLabel;

  /// SKU.
  final String sku;

  /// Ojon-variable.
  final bool variableWeight;

  /// Mot ordered gram (ojon-variable e).
  final int? orderedWeightG;

  /// JSON theke.
  PickVariant.fromJson(Map<String, dynamic> j)
    : variantId = field(j, 'variant_id'),
      productName = LocalizedText.fromJson(_obj(j, 'product_name')),
      variantLabel = LocalizedText.fromJson(_obj(j, 'variant_label')),
      sku = field(j, 'sku'),
      variableWeight = field(j, 'variable_weight'),
      quantity = field(j, 'quantity'),
      orderedWeightG = field(j, 'ordered_weight_g');
}

/// Pick list er area.
class PickArea {
  /// District, area.
  final String district, area;

  /// Variant.
  final List<PickVariant> variants;

  /// JSON theke.
  PickArea.fromJson(Map<String, dynamic> j)
    : district = field(j, 'district'),
      area = field(j, 'area'),
      variants = objects(j, 'variants', PickVariant.fromJson);
}

/// Pick list er fulfillment.
class PickFulfillment {
  /// Id, order, kram, vendor.
  final int id, orderId, seq, vendorId;

  /// Order number, recipient, district, area.
  final String orderNumber, recipientName, district, area;

  /// Confirm kokhon.
  final DateTime confirmedAt;

  /// JSON theke.
  PickFulfillment.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      orderId = field(j, 'order_id'),
      orderNumber = field(j, 'order_number'),
      seq = field(j, 'seq'),
      vendorId = field(j, 'vendor_id'),
      recipientName = field(j, 'recipient_name'),
      district = field(j, 'district'),
      area = field(j, 'area'),
      confirmedAt = time(j, 'confirmed_at');
}

/// `GET /api/admin/inventory/variants` er row (11.7 item 1).
class InventoryVariant {
  /// Variant, product.
  final int variantId, productId;

  /// Product er naam.
  final LocalizedText productName;

  /// Variant er `options` (jemon `weight` → `{bn, en}`).
  final Map<String, LocalizedText> variantLabel;

  /// SKU.
  final String sku;

  /// Stock, low stock er shima, batch e baki.
  final int stock, lowStockAt, batchRemaining;

  /// Shobcheye kacher expiry (din).
  final String? nextExpiry;

  /// JSON theke.
  InventoryVariant.fromJson(Map<String, dynamic> j)
    : variantId = field(j, 'variant_id'),
      productId = field(j, 'product_id'),
      productName = LocalizedText.fromJson(_obj(j, 'product_name')),
      variantLabel = {
        for (final e in _obj(j, 'variant_label').entries)
          e.key: LocalizedText.fromJson(_obj(_obj(j, 'variant_label'), e.key)),
      },
      sku = field(j, 'sku'),
      stock = field(j, 'stock'),
      lowStockAt = field(j, 'low_stock_at'),
      batchRemaining = field(j, 'batch_remaining'),
      nextExpiry = field(j, 'next_expiry');
}

/// Inventory batch (FIFO kram). `staff`/`vendor_staff` e [costPrice] null.
class Batch {
  /// Id, variant, pawa, baki.
  final int id, variantId, quantityReceived, quantityRemaining;

  /// Kena dam.
  final int? costPrice;

  /// Batch code, harvest ar expiry er din.
  final String? batchCode, harvestDate, expiryDate;

  /// Kokhon.
  final DateTime receivedAt;

  /// JSON theke.
  Batch.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      variantId = field(j, 'variant_id'),
      batchCode = field(j, 'batch_code'),
      quantityReceived = field(j, 'quantity_received'),
      quantityRemaining = field(j, 'quantity_remaining'),
      costPrice = field(j, 'cost_price'),
      harvestDate = field(j, 'harvest_date'),
      expiryDate = field(j, 'expiry_date'),
      receivedAt = time(j, 'received_at');
}

/// Stock er movement (notun age). `staff`/`vendor_staff` e [costPrice] null.
class Movement {
  /// Id, variant, bodol (+/-).
  final int id, variantId, change;

  /// Batch, order, staff, kena dam.
  final int? batchId, orderId, staffId, costPrice;

  /// Karon (`purchase`, `order`, `cancel`, `return`, `adjust`, `wastage`, `expired`).
  final String reason;

  /// Note.
  final String? note;

  /// Kokhon.
  final DateTime createdAt;

  /// JSON theke.
  Movement.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      variantId = field(j, 'variant_id'),
      batchId = field(j, 'batch_id'),
      change = field(j, 'change'),
      reason = field(j, 'reason'),
      orderId = field(j, 'order_id'),
      staffId = field(j, 'staff_id'),
      note = field(j, 'note'),
      costPrice = field(j, 'cost_price'),
      createdAt = time(j, 'created_at');
}

/// Coupon (11.7 item 13 v); `vendor_id`/`funded_by` MV e.
class Coupon {
  /// Id, value (percent 1–100 ba poisha), min order, bebohar, phone prati shima.
  final int id, value, minOrder, usedCount, perPhoneLimit;

  /// Code, kind (`percent`, `fixed`).
  final String code, kind;

  /// Max discount, mot shima (null = nai).
  final int? maxDiscount, usageLimit;

  /// Shuru, shesh.
  final DateTime? startsAt, expiresAt;

  /// Active.
  final bool isActive;

  /// Kokhon.
  final DateTime createdAt;

  /// JSON theke.
  Coupon.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      code = field(j, 'code'),
      kind = field(j, 'kind'),
      value = field(j, 'value'),
      minOrder = field(j, 'min_order'),
      maxDiscount = field(j, 'max_discount'),
      usageLimit = field(j, 'usage_limit'),
      usedCount = field(j, 'used_count'),
      perPhoneLimit = field(j, 'per_phone_limit'),
      startsAt = timeOrNull(j, 'starts_at'),
      expiresAt = timeOrNull(j, 'expires_at'),
      isActive = field(j, 'is_active'),
      createdAt = time(j, 'created_at');
}

/// Delivery zone (11.7 item 13 vi); `same_day_cutoff`, `own_rider` Phase 2.
class Zone {
  /// Id, charge, min order.
  final int id, charge, minOrder;

  /// Naam.
  final LocalizedText name;

  /// District.
  final String district;

  /// Area (null = puro district).
  final String? area;

  /// Er beshi hole delivery free (null = nai).
  final int? freeDeliveryOver;

  /// Ek shohor (deadline), active.
  final bool sameCity, isActive;

  /// JSON theke.
  Zone.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      name = LocalizedText.fromJson(_obj(j, 'name')),
      district = field(j, 'district'),
      area = field(j, 'area'),
      charge = field(j, 'charge'),
      minOrder = field(j, 'min_order'),
      freeDeliveryOver = field(j, 'free_delivery_over'),
      sameCity = field(j, 'same_city'),
      isActive = field(j, 'is_active');
}

/// Customer (11.7 item 12): list e [orderCount]/[orderTotal], detail e [orders].
class Customer {
  /// Id.
  final int id;

  /// Phone, bhasha.
  final String phone, locale;

  /// Naam, email.
  final String? name, email;

  /// Kokhon.
  final DateTime createdAt;

  /// Phone block kora.
  final bool blocked;

  /// Shudhu list e: order count, mot (cancelled bad).
  final int? orderCount, orderTotal;

  /// Shudhu detail e: shesh 50 order.
  final List<CustomerOrder> orders;

  /// JSON theke.
  Customer.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      phone = field(j, 'phone'),
      name = field(j, 'name'),
      email = field(j, 'email'),
      locale = field(j, 'locale'),
      createdAt = time(j, 'created_at'),
      blocked = field(j, 'blocked'),
      orderCount = field(j, 'order_count'),
      orderTotal = field(j, 'order_total'),
      orders = objects(j, 'orders', CustomerOrder.fromJson);
}

/// Customer detail er order.
class CustomerOrder {
  /// Id, total.
  final int id, total;

  /// Order number, `display_status`, payment method/status.
  final String orderNumber, displayStatus, paymentMethod, paymentStatus;

  /// Kokhon.
  final DateTime placedAt;

  /// JSON theke.
  CustomerOrder.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      orderNumber = field(j, 'order_number'),
      displayStatus = field(j, 'display_status'),
      paymentMethod = field(j, 'payment_method'),
      paymentStatus = field(j, 'payment_status'),
      total = field(j, 'total'),
      placedAt = time(j, 'placed_at');
}

/// Block kora phone.
class BlockedPhone {
  /// Phone.
  final String phone;

  /// Karon.
  final String? reason;

  /// Ke block korlo.
  final int? staffId;

  /// Kokhon.
  final DateTime createdAt;

  /// JSON theke.
  BlockedPhone.fromJson(Map<String, dynamic> j)
    : phone = field(j, 'phone'),
      reason = field(j, 'reason'),
      staffId = field(j, 'staff_id'),
      createdAt = time(j, 'created_at');
}

/// Audit log er row (11.7 item 11); CLI er row e staff null.
class AuditEntry {
  /// Id.
  final int id;

  /// Staff, entity.
  final int? staffId, entityId;

  /// Staff er naam, ip.
  final String? staffName, ip;

  /// Action (`staff.reset` ...), entity type.
  final String action, entityType;

  /// Ager ar porer JSON (je kono shape).
  final Object? before, after;

  /// Kokhon.
  final DateTime createdAt;

  /// JSON theke.
  AuditEntry.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      staffId = field(j, 'staff_id'),
      staffName = field(j, 'staff_name'),
      action = field(j, 'action'),
      entityType = field(j, 'entity_type'),
      entityId = field(j, 'entity_id'),
      before = j['before'],
      after = j['after'],
      ip = field(j, 'ip'),
      createdAt = time(j, 'created_at');
}

/// `GET /api/admin/dashboard` (11.7 item 13 viii, v1.46): shudhu count.
class Dashboard {
  /// Aj (shop timezone).
  final String date;

  /// Count; [sales] poisha.
  final int orders, sales, pending, toPack, lowStock, expiring, deadlineSoon;

  /// JSON theke.
  Dashboard.fromJson(Map<String, dynamic> j)
    : date = field(j, 'date'),
      orders = field(j, 'orders'),
      sales = field(j, 'sales'),
      pending = field(j, 'pending'),
      toPack = field(j, 'to_pack'),
      lowStock = field(j, 'low_stock'),
      expiring = field(j, 'expiring'),
      deadlineSoon = field(j, 'deadline_soon');
}

/// Upload kora product er chobi (11.7 item 9).
class AdminImage {
  /// Id, product, pixel, kram.
  final int id, productId, width, height, sortOrder;

  /// Storage key.
  final String storageKey;

  /// Alt.
  final LocalizedText alt;

  /// JSON theke.
  AdminImage.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      productId = field(j, 'product_id'),
      storageKey = field(j, 'storage_key'),
      alt = LocalizedText.fromJson(_obj(j, 'alt')),
      width = field(j, 'width'),
      height = field(j, 'height'),
      sortOrder = field(j, 'sort_order');
}

/// Admin category (`GET /api/admin/categories`, inactive o; 6.7).
class AdminCategory {
  /// Id, parent (root e null).
  final int id;

  /// Parent category.
  final int? parentId;

  /// URL er slug.
  final String slug;

  /// Naam `{bn, en}`.
  final LocalizedText name;

  /// Chobi (storage key).
  final String? imageKey;

  /// Kram.
  final int sortOrder;

  /// Website e dekhay.
  final bool isActive;

  /// JSON theke.
  AdminCategory.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      parentId = field(j, 'parent_id'),
      slug = field(j, 'slug'),
      name = LocalizedText.fromJson(_obj(j, 'name')),
      imageKey = field(j, 'image_key'),
      sortOrder = field(j, 'sort_order'),
      isActive = field(j, 'is_active');
}

/// Admin product (`to_jsonb` row, 6.7): list e [variants]/[images] khali, detail e shob. SEO ar `attributes` ekhon
/// form e na (PATCH e na pathale bodlay na).
class AdminProduct {
  /// Id, vendor.
  final int id, vendorId;

  /// Category.
  final int? categoryId;

  /// Slug, unit (`piece`, `kg` ...), khojar shobdo.
  final String slug, unit, searchKeywords;

  /// Naam, bornona.
  final LocalizedText name, description;

  /// Ojon onujayi dam (unit `kg`), website e dekhay.
  final bool variableWeight, isActive;

  /// Variant (sort_order kram).
  final List<AdminVariant> variants;

  /// Chobi (sort_order kram).
  final List<AdminImage> images;

  /// JSON theke.
  AdminProduct.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      vendorId = field(j, 'vendor_id'),
      categoryId = field(j, 'category_id'),
      slug = field(j, 'slug'),
      name = LocalizedText.fromJson(_obj(j, 'name')),
      description = LocalizedText.fromJson(_obj(j, 'description')),
      unit = field(j, 'unit'),
      variableWeight = field(j, 'variable_weight'),
      searchKeywords = field(j, 'search_keywords'),
      isActive = field(j, 'is_active'),
      variants = objects(j, 'variants', AdminVariant.fromJson),
      images = objects(j, 'images', AdminImage.fromJson);
}

/// Product variant (6.7); taka poisha `int`; stock shudhu batch/adjust e bodlay (8.12).
class AdminVariant {
  /// Id, product.
  final int id, productId;

  /// SKU.
  final String sku;

  /// Option (jemon `weight` → `{bn, en}`).
  final Map<String, LocalizedText> options;

  /// Dam (variable weight e 1 kg er), stock, low stock er shima, kram.
  final int price, stock, lowStockAt, sortOrder;

  /// Ager dam, kena dam, ek unit er ojon (gram).
  final int? compareAtPrice, costPrice, nominalWeightG;

  /// Website e dekhay.
  final bool isActive;

  /// JSON theke.
  AdminVariant.fromJson(Map<String, dynamic> j)
    : id = field(j, 'id'),
      productId = field(j, 'product_id'),
      sku = field(j, 'sku'),
      options = {
        for (final e in _obj(j, 'options').entries)
          e.key: LocalizedText.fromJson(e.value as Map<String, dynamic>),
      },
      price = field(j, 'price'),
      compareAtPrice = field(j, 'compare_at_price'),
      costPrice = field(j, 'cost_price'),
      nominalWeightG = field(j, 'nominal_weight_g'),
      stock = field(j, 'stock'),
      lowStockAt = field(j, 'low_stock_at'),
      isActive = field(j, 'is_active'),
      sortOrder = field(j, 'sort_order');
}
