import 'json.dart';

/// Cart er ek line (7.4). Dam poisha, protiti GET e fresh (8.4); `in_stock` na hole subtotal e na.
class CartItem {
  /// Variant id.
  final int variantId;

  /// Product er slug, naam (locale e), variant er lekha (jemon "৫ কেজি", khali hote pare).
  final String productSlug, name, variantLabel;

  /// Prothom chobi (storage key).
  final String? image;

  /// Ek unit er dam (variable_weight hole 1 kg er).
  final int unitPrice;

  /// Ager dam.
  final int? compareAtPrice;

  /// Kg er dam (line total approx, 8.5).
  final bool variableWeight;

  /// Quantity ar tar shima `min(stock, 50)`.
  final int quantity, maxQuantity;

  /// Stock ache.
  final bool inStock;

  /// Line total (Go er `pricing`; stock na thakle 0).
  final int lineTotal;

  /// CartItem.
  const CartItem(
    this.variantId,
    this.productSlug,
    this.name,
    this.variantLabel,
    this.image,
    this.unitPrice,
    this.compareAtPrice,
    this.variableWeight,
    this.quantity,
    this.maxQuantity,
    this.inStock,
    this.lineTotal,
  );

  /// JSON theke.
  factory CartItem.fromJson(Map<String, dynamic> j) => CartItem(
    field(j, 'variant_id'),
    field(j, 'product_slug'),
    field(j, 'name'),
    field(j, 'variant_label'),
    field(j, 'image'),
    field(j, 'unit_price'),
    field(j, 'compare_at_price'),
    field(j, 'variable_weight'),
    field(j, 'quantity'),
    field(j, 'max_quantity'),
    field(j, 'in_stock'),
    field(j, 'line_total'),
  );
}

/// Cart er notice (8.4): `item_removed`, `quantity_reduced`, verify er `item_dropped`; lekha
/// `cart.notice.<code>` (`{name}`).
class CartNotice {
  /// Code.
  final String code;

  /// Product er naam (request er bhashay).
  final String name;

  /// CartNotice.
  const CartNotice(this.code, this.name);

  /// JSON theke.
  factory CartNotice.fromJson(Map<String, dynamic> j) =>
      CartNotice(field(j, 'code'), field(j, 'name'));
}

/// Puro cart (`GET`/`POST`/`PATCH`/`DELETE /api/cart...`, 7.4).
class Cart {
  /// Line.
  final List<CartItem> items;

  /// Alada line er shonkhya (qty jog na).
  final int count;

  /// Subtotal (poisha, shudhu stock er line).
  final int subtotal;

  /// Ei response er notice (ekbar-i ashe).
  final List<CartNotice> notices;

  /// Notun guest cart er key (shudhu prothom add e); website e `__Host-cart` e jay.
  final String? cartKey;

  /// Cart.
  const Cart(this.items, this.count, this.subtotal, this.notices, this.cartKey);

  /// JSON theke.
  factory Cart.fromJson(Map<String, dynamic> j) => Cart(
    objects(j, 'items', CartItem.fromJson),
    field(j, 'count'),
    field(j, 'subtotal'),
    objects(j, 'notices', CartNotice.fromJson),
    field(j, 'cart_key'),
  );
}
