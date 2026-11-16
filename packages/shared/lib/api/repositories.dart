import '../models/cart.dart';
import '../models/catalog.dart';
import '../models/order.dart';
import '../models/page.dart';
import '../models/settings.dart';
import '../models/user.dart';
import '../models/json.dart';
import 'api_client.dart';

/// Public catalog (7.2): category, product list (filter, search, paging), detail, shop er policy.
class CatalogRepository {
  /// Client.
  final ApiClient api;

  /// CatalogRepository.
  const CatalogRepository(this.api);

  /// `GET /api/categories` (tree).
  Future<List<Category>> categories() async => [
    for (final c in await api.get('/api/categories') as List<dynamic>)
      Category.fromJson(c as Map<String, dynamic>),
  ];

  /// `GET /api/products`; dam poisha. Na deya filter query te jay na.
  Future<Page<ProductCard>> products({
    String? q,
    String? category,
    String? vendor,
    int? minPrice,
    int? maxPrice,
    bool? inStock,
    String? sort,
    int? page,
    int? limit,
  }) async {
    final query = {
      'q': q,
      'category': category,
      'vendor': vendor,
      'min_price': minPrice?.toString(),
      'max_price': maxPrice?.toString(),
      'in_stock': inStock?.toString(),
      'sort': sort,
      'page': page?.toString(),
      'limit': limit?.toString(),
    }..removeWhere((_, v) => v == null);
    final j =
        await api.get('/api/products', query: query.cast<String, String>())
            as Map<String, dynamic>;
    return Page.fromJson(j, ProductCard.fromJson);
  }

  /// `GET /api/products/{slug}`; nai hole `ApiError` `not_found`.
  Future<Product> product(String slug) async => Product.fromJson(
    await api.get('/api/products/${Uri.encodeComponent(slug)}')
        as Map<String, dynamic>,
  );

  /// `GET /api/policies/{kind}` er `body` (plain text; locale er, na hole Bangla, v1.13); ochena kind `not_found`.
  Future<String> policy(String kind) async =>
      (await api.get('/api/policies/${Uri.encodeComponent(kind)}')
              as Map<String, dynamic>)['body']
          as String;
}

/// Shop er public settings ar translation override (7.2, 9.8).
class SettingsRepository {
  /// Client.
  final ApiClient api;

  /// SettingsRepository.
  const SettingsRepository(this.api);

  /// `GET /api/settings`.
  Future<PublicSettings> settings() async => PublicSettings.fromJson(
    await api.get('/api/settings') as Map<String, dynamic>,
  );

  /// `GET /api/settings/version` (cache check).
  Future<int> version() async =>
      (await api.get('/api/settings/version')
              as Map<String, dynamic>)['version']
          as int;

  /// `GET /api/translations?locale=` → `Translator` er `overrides[locale]`.
  Future<Map<String, String>> translations(String locale) async =>
      (await api.get('/api/translations', query: {'locale': locale})
              as Map<String, dynamic>)
          .cast<String, String>();
}

/// Customer er account (7.3); [api] e `token` lage.
class AccountRepository {
  /// Client.
  final ApiClient api;

  /// AccountRepository.
  const AccountRepository(this.api);

  /// `GET /api/me`; session na thakle `ApiError` `401`.
  Future<User> me() async =>
      User.fromJson(await api.get('/api/me') as Map<String, dynamic>);
}

/// Cart (7.4): login (`token`) thakle account cart, na hole `cartKey` er guest cart. Shob route puro
/// [Cart] ferot dey; dam ar total shudhu server er (Rule 1).
class CartRepository {
  /// Client.
  final ApiClient api;

  /// CartRepository.
  const CartRepository(this.api);

  Future<Cart> _send(String method, String path, [Object? body]) async =>
      Cart.fromJson(
        await api.send(method, path, body: body) as Map<String, dynamic>,
      );

  /// `GET /api/cart` (kichu banay na).
  Future<Cart> cart() => _send('GET', '/api/cart');

  /// `GET /api/cart/count` (badge, 7.4 v1.31): alada line, normalize/notice na.
  Future<int> count() async => field(
    (await api.get('/api/cart/count')) as Map<String, dynamic>,
    'count',
  );

  /// `POST /api/cart/items`; ekoi variant e qty jog. Beshi hole `ApiError` `out_of_stock`
  /// (`details.max_quantity`), 50 variant e `cart_full`.
  Future<Cart> add(int variantId, int quantity) => _send(
    'POST',
    '/api/cart/items',
    {'variant_id': variantId, 'quantity': quantity},
  );

  /// `PATCH /api/cart/items/{variant_id}`.
  Future<Cart> setQuantity(int variantId, int quantity) =>
      _send('PATCH', '/api/cart/items/$variantId', {'quantity': quantity});

  /// `DELETE /api/cart/items/{variant_id}` (na thakleo thik).
  Future<Cart> remove(int variantId) =>
      _send('DELETE', '/api/cart/items/$variantId');
}

/// Wishlist (7.4): shob route login chay (`token`); shob route product card er list ferot dey.
class WishlistRepository {
  /// Client.
  final ApiClient api;

  /// WishlistRepository.
  const WishlistRepository(this.api);

  Future<List<ProductCard>> _send(
    String method,
    String path, [
    Object? body,
  ]) async => objects(
    await api.send(method, path, body: body) as Map<String, dynamic>,
    'items',
    ProductCard.fromJson,
  );

  /// `GET /api/wishlist`.
  Future<List<ProductCard>> wishlist() => _send('GET', '/api/wishlist');

  /// `POST /api/wishlist/{product_id}`; 200 thakle `ApiError` `wishlist_full`.
  Future<List<ProductCard>> add(int productId) =>
      _send('POST', '/api/wishlist/$productId');

  /// `DELETE /api/wishlist/{product_id}`.
  Future<List<ProductCard>> remove(int productId) =>
      _send('DELETE', '/api/wishlist/$productId');

  /// `POST /api/wishlist/merge` (guest er `wl` cookie er ID, login er por).
  Future<List<ProductCard>> merge(List<int> productIds) =>
      _send('POST', '/api/wishlist/merge', {'product_ids': productIds});
}

/// Checkout (7.2, 7.5, 8.6): login (`token`) ba guest cart (`cartKey`). Body 7.5 er field (BFF hubohu
/// forward kore); dam/total er field nai, server hishab kore (Rule 1).
class CheckoutRepository {
  /// Client.
  final ApiClient api;

  /// CheckoutRepository.
  const CheckoutRepository(this.api);

  /// `GET /api/delivery-zones`.
  Future<List<DeliveryZone>> zones() async => [
    for (final z in await api.get('/api/delivery-zones') as List<dynamic>)
      DeliveryZone.fromJson(z as Map<String, dynamic>),
  ];

  /// `POST /api/checkout/quote` (`district`, `area`, `phone`, `payment_method`, `coupon_code`).
  Future<Quote> quote(Map<String, Object?> body) async => Quote.fromJson(
    await api.send('POST', '/api/checkout/quote', body: body)
        as Map<String, dynamic>,
  );

  /// `POST /api/orders`; ekoi body er retry e ekoi [idempotencyKey] (8.6 step 1). `409 otp_required`
  /// (`details.phone_masked`), `price_changed` (`details.total` ...) `ApiError` e.
  Future<PlacedOrder> placeOrder(
    Map<String, Object?> body, {
    required String idempotencyKey,
  }) async => PlacedOrder.fromJson(
    await api.send(
          'POST',
          '/api/orders',
          body: body,
          headers: {'Idempotency-Key': idempotencyKey},
        )
        as Map<String, dynamic>,
  );
}

/// Order (7.3, 7.5, 8.10): token er page, cancel, tracking, COD confirm, account er order.
class OrderRepository {
  /// Client.
  final ApiClient api;

  /// OrderRepository.
  const OrderRepository(this.api);

  Future<String> _status(String path, [Object? body]) async => field(
    (await api.send('POST', path, body: body)) as Map<String, dynamic>,
    'status',
  );

  /// `GET /api/orders/{token}` (Rule 11); na mille `not_found`.
  Future<Order> byToken(String token) async => Order.fromJson(
    await api.get('/api/orders/${Uri.encodeComponent(token)}')
        as Map<String, dynamic>,
  );

  /// `POST /api/orders/{token}/cancel`; notun status. Na hole `invalid_transition`.
  Future<String> cancel(String token, {String? reason}) => _status(
    '/api/orders/${Uri.encodeComponent(token)}/cancel',
    {'reason': ?reason}, // server JSON body chay, khali hole-o `{}`
  );

  /// `POST /api/orders/track` (number ar phone body te, URL e na).
  Future<TrackedOrder> track(String number, String phone) async =>
      TrackedOrder.fromJson(
        await api.send(
              'POST',
              '/api/orders/track',
              body: {'number': number, 'phone': phone},
            )
            as Map<String, dynamic>,
      );

  /// `GET /api/confirm/{confirm_token}` (8.10).
  Future<ConfirmSummary> confirmView(String confirmToken) async =>
      ConfirmSummary.fromJson(
        await api.get('/api/confirm/${Uri.encodeComponent(confirmToken)}')
            as Map<String, dynamic>,
      );

  /// `POST /api/confirm/{confirm_token}`; abar hole `invalid_transition`.
  Future<String> confirm(String confirmToken) =>
      _status('/api/confirm/${Uri.encodeComponent(confirmToken)}');

  /// `GET /api/me/orders` (`token` lage); na deya page/limit query te jay na.
  Future<Page<OrderSummary>> myOrders({int? page, int? limit}) async {
    final query = {'page': ?page?.toString(), 'limit': ?limit?.toString()};
    return Page.fromJson(
      await api.get('/api/me/orders', query: query.isEmpty ? null : query)
          as Map<String, dynamic>,
      OrderSummary.fromJson,
    );
  }
}
