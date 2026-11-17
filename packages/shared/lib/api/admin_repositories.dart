import 'package:http/http.dart' as http;

import '../models/admin.dart';
import '../models/json.dart';
import '../models/page.dart';
import 'api_client.dart';

// Admin API (7.7, 11.7): staff session `__Host-asid` cookie e (11.3), tai client e token nai; admin `BrowserClient`
// `withCredentials` diye. Din (`from`, `to`, `date`, batch er din) `YYYY-MM-DD` shop er timezone e.

/// Null bad, baki `toString` (query).
Map<String, String>? _query(Map<String, Object?> q) {
  final m = {
    for (final e in q.entries)
      if (e.value != null) e.key: '${e.value}',
  };
  return m.isEmpty ? null : m;
}

Future<Map<String, dynamic>> _json(
  ApiClient api,
  String method,
  String path, [
  Object? body,
]) async => await api.send(method, path, body: body) as Map<String, dynamic>;

Future<Page<T>> _page<T>(
  ApiClient api,
  String path,
  T Function(Map<String, dynamic>) item,
  Map<String, Object?> query,
) async => Page.fromJson(
  await api.get(path, query: _query(query)) as Map<String, dynamic>,
  item,
);

/// Status bodol er uttor (`status` ar order er `display_status`).
typedef StatusResult = ({String status, String displayStatus});

StatusResult _status(Map<String, dynamic> j) =>
    (status: field(j, 'status'), displayStatus: field(j, 'display_status'));

/// Staff login (8.3, 7.7).
class AdminAuthRepository {
  /// Client.
  final ApiClient api;

  /// AdminAuthRepository.
  const AdminAuthRepository(this.api);

  /// `POST /api/admin/auth/login` (`shop` = slug): TOTP lage (`tempToken`) ba prothom setup (`setup_pending` cookie).
  Future<({bool totpRequired, String? tempToken, bool setupRequired})> login(
    String shop,
    String email,
    String password,
  ) async {
    final j = await _json(api, 'POST', '/api/admin/auth/login', {
      'shop': shop,
      'email': email,
      'password': password,
    });
    return (
      totpRequired: field<bool?>(j, 'totp_required') ?? false,
      tempToken: field<String?>(j, 'temp_token'),
      setupRequired: field<bool?>(j, 'setup_required') ?? false,
    );
  }

  Future<bool> _mfa(String path, String tempToken, String code) async => field(
    await _json(api, 'POST', path, {'temp_token': tempToken, 'code': code}),
    'setup_required',
  );

  /// `POST /api/admin/auth/totp` → session cookie; true hole password bodol/setup baki.
  Future<bool> totp(String tempToken, String code) =>
      _mfa('/api/admin/auth/totp', tempToken, code);

  /// `POST /api/admin/auth/recovery` (TOTP er bodole recovery code).
  Future<bool> recovery(String tempToken, String code) =>
      _mfa('/api/admin/auth/recovery', tempToken, code);

  /// `POST /api/admin/auth/password` → `204`.
  Future<void> changePassword(String old, String new_) => api.send(
    'POST',
    '/api/admin/auth/password',
    body: {'old': old, 'new': new_},
  );

  /// `POST /api/admin/auth/totp/setup` (pending secret).
  Future<({String otpauthUrl, String secret})> totpSetup() async {
    final j = await _json(api, 'POST', '/api/admin/auth/totp/setup');
    return (
      otpauthUrl: field<String>(j, 'otpauth_url'),
      secret: field<String>(j, 'secret'),
    );
  }

  /// `POST /api/admin/auth/totp/enable` → recovery code (ekbar dekhano).
  Future<List<String>> totpEnable(String code) async => field<List<dynamic>>(
    await _json(api, 'POST', '/api/admin/auth/totp/enable', {'code': code}),
    'recovery_codes',
  ).cast<String>();

  /// `POST /api/admin/auth/logout` → `204`.
  Future<void> logout() => api.send('POST', '/api/admin/auth/logout');

  /// `GET /api/admin/me`.
  Future<StaffProfile> me() async =>
      StaffProfile.fromJson(await _json(api, 'GET', '/api/admin/me'));
}

/// Order, fulfillment, packing, shipment, print (7.7, 8.8, 8.11, 11.7 item 4–8).
class AdminOrderRepository {
  /// Client.
  final ApiClient api;

  /// AdminOrderRepository.
  const AdminOrderRepository(this.api);

  /// `GET /api/admin/orders`; `status` = `display_status`, `tab` `awaiting_confirm`/`deadline_soon`, `q` number ba phone.
  Future<Page<AdminOrder>> orders({
    String? status,
    String? tab,
    bool? risk,
    String? q,
    String? from,
    String? to,
    int? vendorId,
    int? page,
    int? limit,
  }) => _page(api, '/api/admin/orders', AdminOrder.fromJson, {
    'status': status,
    'tab': tab,
    'risk': risk == null ? null : (risk ? '1' : '0'),
    'q': q,
    'from': from,
    'to': to,
    'vendor_id': vendorId,
    'page': page,
    'limit': limit,
  });

  /// `GET /api/admin/orders/{id}`.
  Future<OrderDetail> order(int id) async =>
      OrderDetail.fromJson(await _json(api, 'GET', '/api/admin/orders/$id'));

  /// `POST /api/admin/orders/{id}/transition` (`confirmed`, `cancelled`).
  Future<StatusResult> transition(int id, String to, {String? note}) async =>
      _status(
        await _json(api, 'POST', '/api/admin/orders/$id/transition', {
          'to': to,
          'note': ?note,
        }),
      );

  /// `POST /api/admin/orders/{id}/notes` (`kind` `note` ba `call`).
  Future<OrderNote> addNote(
    int id,
    String note, {
    String kind = 'note',
  }) async => OrderNote.fromJson(
    await _json(api, 'POST', '/api/admin/orders/$id/notes', {
      'kind': kind,
      'note': note,
    }),
  );

  /// `PATCH /api/admin/orders/{id}/items`: item id → notun qty (shudhu kom, `0` = bad); note lagbe.
  Future<StatusResult> editItems(
    int id,
    Map<int, int> quantities,
    String note,
  ) async => _status(
    await _json(api, 'PATCH', '/api/admin/orders/$id/items', {
      'items': [
        for (final e in quantities.entries)
          {'item_id': e.key, 'quantity': e.value},
      ],
      'note': note,
    }),
  );

  /// `GET /api/admin/fulfillments` (`date` = order er din).
  Future<Page<FulfillmentRow>> fulfillments({
    String? status,
    int? vendorId,
    String? date,
    int? page,
    int? limit,
  }) => _page(api, '/api/admin/fulfillments', FulfillmentRow.fromJson, {
    'status': status,
    'vendor_id': vendorId,
    'date': date,
    'page': page,
    'limit': limit,
  });

  /// `POST /api/admin/fulfillments/{id}/transition` (`open`, `returned`, `cancelled`); notun status.
  Future<String> transitionFulfillment(
    int id,
    String to, {
    String? note,
    List<int>? restockItemIds,
  }) async => field(
    await _json(api, 'POST', '/api/admin/fulfillments/$id/transition', {
      'to': to,
      'note': ?note,
      'restock_item_ids': ?restockItemIds,
    }),
    'status',
  );

  /// `POST /api/admin/fulfillments/{id}/pack`: ojon-variable item id → packed gram; COD +10% er beshi hole
  /// `409 weight_over_limit`, customer raji hole [customerOkNote].
  Future<StatusResult> pack(
    int id,
    Map<int, int> weights, {
    String? customerOkNote,
  }) async => _status(
    await _json(api, 'POST', '/api/admin/fulfillments/$id/pack', {
      'items': [
        for (final e in weights.entries)
          {'item_id': e.key, 'packed_weight_g': e.value},
      ],
      'customer_ok_note': ?customerOkNote,
    }),
  );

  /// `POST /api/admin/fulfillments/{id}/shipments` (`carrier`: `own_rider`, `steadfast`, `pathao`, `redx`, `other`).
  Future<Shipment> ship(
    int fulfillmentId,
    String carrier, {
    int? riderStaffId,
    String? trackingCode,
    int? carrierFee,
  }) async => Shipment.fromJson(
    await _json(
      api,
      'POST',
      '/api/admin/fulfillments/$fulfillmentId/shipments',
      {
        'carrier': carrier,
        'rider_staff_id': ?riderStaffId,
        'tracking_code': ?trackingCode,
        'carrier_fee': ?carrierFee,
      },
    ),
  );

  /// `PATCH /api/admin/shipments/{id}` (`delivered`, `returned`, `cancelled`, ba shudhu tracking).
  Future<Shipment> updateShipment(
    int id, {
    String? status,
    String? trackingCode,
  }) async => Shipment.fromJson(
    await _json(api, 'PATCH', '/api/admin/shipments/$id', {
      'status': ?status,
      'tracking_code': ?trackingCode,
    }),
  );

  /// `GET /api/admin/packing/pick-list` (na dile aj).
  Future<PickList> pickList({String? date}) async => PickList.fromJson(
    await api.get('/api/admin/packing/pick-list', query: _query({'date': date}))
        as Map<String, dynamic>,
  );

  /// `POST /api/admin/print-links`: `packing_slip`/`invoice` e [orderIds], `pick_list` e [date] (na dile aj);
  /// [url] notun tab e (10 minute).
  Future<({String url, DateTime expiresAt})> printLink(
    String kind, {
    List<int>? orderIds,
    String? date,
  }) async {
    final j = await _json(api, 'POST', '/api/admin/print-links', {
      'kind': kind,
      'order_ids': ?orderIds,
      'date': ?date,
    });
    return (url: field<String>(j, 'url'), expiresAt: time(j, 'expires_at'));
  }
}

/// Inventory (8.12, 11.7 item 1, 2).
class InventoryRepository {
  /// Client.
  final ApiClient api;

  /// InventoryRepository.
  const InventoryRepository(this.api);

  /// `GET /api/admin/inventory/variants` (`q` naam ba SKU, `low` = stock ≤ shima).
  Future<Page<InventoryVariant>> variants({
    String? q,
    bool? low,
    int? page,
    int? limit,
  }) => _page(api, '/api/admin/inventory/variants', InventoryVariant.fromJson, {
    'q': q,
    'low': low == null ? null : (low ? '1' : '0'),
    'page': page,
    'limit': limit,
  });

  /// `GET /api/admin/variants/{id}/batches` (FIFO kram).
  Future<Page<Batch>> batches(int variantId, {int? page, int? limit}) => _page(
    api,
    '/api/admin/variants/$variantId/batches',
    Batch.fromJson,
    {'page': page, 'limit': limit},
  );

  /// `POST /api/admin/batches` (stock barano shudhu eta).
  Future<Batch> createBatch(
    int variantId,
    int quantity, {
    int? costPrice,
    String? batchCode,
    String? harvestDate,
    String? expiryDate,
  }) async => Batch.fromJson(
    await _json(api, 'POST', '/api/admin/batches', {
      'variant_id': variantId,
      'quantity': quantity,
      'cost_price': ?costPrice,
      'batch_code': ?batchCode,
      'harvest_date': ?harvestDate,
      'expiry_date': ?expiryDate,
    }),
  );

  /// `POST /api/admin/inventory/adjust` (shudhu komano, `change < 0`; `reason` `wastage` ba `adjust`); notun stock.
  Future<int> adjust(
    int variantId,
    int change,
    String reason,
    String note, {
    int? batchId,
  }) async => field(
    await _json(api, 'POST', '/api/admin/inventory/adjust', {
      'variant_id': variantId,
      'change': change,
      'reason': reason,
      'note': note,
      'batch_id': ?batchId,
    }),
    'stock',
  );

  /// `GET /api/admin/inventory/movements` (notun age).
  Future<Page<Movement>> movements({int? variantId, int? page, int? limit}) =>
      _page(api, '/api/admin/inventory/movements', Movement.fromJson, {
        'variant_id': variantId,
        'page': page,
        'limit': limit,
      });
}

/// Product, variant, category (7.7) ar chobi (11.7 item 9). Body 6.7 er field (`name` `{bn, en}`, taka poisha);
/// PATCH e shudhu deya field bodlay, `null` = muche dao.
class AdminCatalogRepository {
  /// Client.
  final ApiClient api;

  /// AdminCatalogRepository.
  const AdminCatalogRepository(this.api);

  /// `GET /api/admin/categories`: shob (inactive o), array (paging na).
  Future<List<AdminCategory>> categories() async => [
    for (final c in await api.get('/api/admin/categories') as List<dynamic>)
      AdminCategory.fromJson(c as Map<String, dynamic>),
  ];

  /// `POST /api/admin/categories`; slug duplicate `409 slug_taken`.
  Future<AdminCategory> createCategory(Map<String, Object?> body) async =>
      AdminCategory.fromJson(
        await _json(api, 'POST', '/api/admin/categories', body),
      );

  /// `PATCH /api/admin/categories/{id}` (parent cycle `400`).
  Future<AdminCategory> updateCategory(
    int id,
    Map<String, Object?> body,
  ) async => AdminCategory.fromJson(
    await _json(api, 'PATCH', '/api/admin/categories/$id', body),
  );

  /// `DELETE /api/admin/categories/{id}`; product ba child thakle `409 category_not_empty`.
  Future<void> deleteCategory(int id) =>
      api.send('DELETE', '/api/admin/categories/$id');

  /// `GET /api/admin/products` (notun age; vendor role e nijer).
  Future<Page<AdminProduct>> products({int? page, int? limit}) => _page(
    api,
    '/api/admin/products',
    AdminProduct.fromJson,
    {'page': page, 'limit': limit},
  );

  /// `GET /api/admin/products/{id}`: variant ar chobi shoho.
  Future<AdminProduct> product(int id) async => AdminProduct.fromJson(
    await api.get('/api/admin/products/$id') as Map<String, dynamic>,
  );

  /// `POST /api/admin/products` (shop staff e house vendor); slug duplicate `409 slug_taken`.
  Future<AdminProduct> createProduct(Map<String, Object?> body) async =>
      AdminProduct.fromJson(
        await _json(api, 'POST', '/api/admin/products', body),
      );

  /// `PATCH /api/admin/products/{id}` (delete na, `is_active`).
  Future<AdminProduct> updateProduct(int id, Map<String, Object?> body) async =>
      AdminProduct.fromJson(
        await _json(api, 'PATCH', '/api/admin/products/$id', body),
      );

  /// `POST /api/admin/products/{id}/variants` (stock na, 8.12); SKU duplicate `409 sku_taken`.
  Future<AdminVariant> createVariant(
    int productId,
    Map<String, Object?> body,
  ) async => AdminVariant.fromJson(
    await _json(api, 'POST', '/api/admin/products/$productId/variants', body),
  );

  /// `PATCH /api/admin/variants/{id}`.
  Future<AdminVariant> updateVariant(int id, Map<String, Object?> body) async =>
      AdminVariant.fromJson(
        await _json(api, 'PATCH', '/api/admin/variants/$id', body),
      );

  /// `POST /api/admin/products/{id}/images` multipart (`file` JPEG/PNG ≤ 10 MB, `alt_bn`, `alt_en`).
  Future<AdminImage> uploadImage(
    int productId,
    List<int> bytes,
    String filename, {
    String? altBn,
    String? altEn,
  }) async => AdminImage.fromJson(
    await api.multipart(
          '/api/admin/products/$productId/images',
          fields: {'alt_bn': ?altBn, 'alt_en': ?altEn},
          files: [
            http.MultipartFile.fromBytes('file', bytes, filename: filename),
          ],
        )
        as Map<String, dynamic>,
  );

  /// `PATCH /api/admin/products/{id}/images/order`: product er shob chobi er id, notun kram e.
  Future<void> orderImages(int productId, List<int> ids) => api.send(
    'PATCH',
    '/api/admin/products/$productId/images/order',
    body: {'ids': ids},
  );

  /// `DELETE /api/admin/images/{id}`.
  Future<void> deleteImage(int id) =>
      api.send('DELETE', '/api/admin/images/$id');
}

/// Delivery zone (owner/admin, 11.7 item 13 vi). Body 6.7 er field (`name`, `district`, `area`, `charge` ...).
class AdminZoneRepository {
  /// Client.
  final ApiClient api;

  /// AdminZoneRepository.
  const AdminZoneRepository(this.api);

  /// `GET /api/admin/delivery-zones` (inactive o).
  Future<Page<Zone>> zones({int? page, int? limit}) => _page(
    api,
    '/api/admin/delivery-zones',
    Zone.fromJson,
    {'page': page, 'limit': limit},
  );

  /// `POST /api/admin/delivery-zones`; duplicate `409 zone_exists`.
  Future<Zone> create(Map<String, Object?> body) async => Zone.fromJson(
    await _json(api, 'POST', '/api/admin/delivery-zones', body),
  );

  /// `PATCH /api/admin/delivery-zones/{id}` (shudhu bodla field).
  Future<Zone> update(int id, Map<String, Object?> body) async => Zone.fromJson(
    await _json(api, 'PATCH', '/api/admin/delivery-zones/$id', body),
  );
}

/// Coupon (owner/admin, 11.7 item 13 v). Body 6.7 er field (`code`, `kind`, `value` ...).
class AdminCouponRepository {
  /// Client.
  final ApiClient api;

  /// AdminCouponRepository.
  const AdminCouponRepository(this.api);

  /// `GET /api/admin/coupons` (notun age).
  Future<Page<Coupon>> coupons({int? page, int? limit}) => _page(
    api,
    '/api/admin/coupons',
    Coupon.fromJson,
    {'page': page, 'limit': limit},
  );

  /// `POST /api/admin/coupons`; duplicate `409 coupon_exists`.
  Future<Coupon> create(Map<String, Object?> body) async =>
      Coupon.fromJson(await _json(api, 'POST', '/api/admin/coupons', body));

  /// `PATCH /api/admin/coupons/{id}`; bebohar hole value lock (`400 fields.<f>: locked`).
  Future<Coupon> update(int id, Map<String, Object?> body) async =>
      Coupon.fromJson(
        await _json(api, 'PATCH', '/api/admin/coupons/$id', body),
      );
}

/// Customer ar phone block (owner/admin, 11.7 item 12).
class AdminCustomerRepository {
  /// Client.
  final ApiClient api;

  /// AdminCustomerRepository.
  const AdminCustomerRepository(this.api);

  /// `GET /api/admin/customers` (`q` phone ba naam).
  Future<Page<Customer>> customers({String? q, int? page, int? limit}) => _page(
    api,
    '/api/admin/customers',
    Customer.fromJson,
    {'q': q, 'page': page, 'limit': limit},
  );

  /// `GET /api/admin/customers/{id}` (shesh 50 order).
  Future<Customer> customer(int id) async =>
      Customer.fromJson(await _json(api, 'GET', '/api/admin/customers/$id'));

  /// `GET /api/admin/blocked-phones`.
  Future<Page<BlockedPhone>> blockedPhones({int? page, int? limit}) => _page(
    api,
    '/api/admin/blocked-phones',
    BlockedPhone.fromJson,
    {'page': page, 'limit': limit},
  );

  /// `POST /api/admin/blocked-phones` (already block hole ager row).
  Future<BlockedPhone> block(String phone, {String? reason}) async =>
      BlockedPhone.fromJson(
        await _json(api, 'POST', '/api/admin/blocked-phones', {
          'phone': phone,
          'reason': ?reason,
        }),
      );

  /// `DELETE /api/admin/blocked-phones` (phone body te, URL/log e na).
  Future<void> unblock(String phone) =>
      api.send('DELETE', '/api/admin/blocked-phones', body: {'phone': phone});
}

/// Staff (owner, 11.7 item 11).
class StaffRepository {
  /// Client.
  final ApiClient api;

  /// StaffRepository.
  const StaffRepository(this.api);

  /// `GET /api/admin/staff`.
  Future<Page<Staff>> staff({int? page, int? limit}) => _page(
    api,
    '/api/admin/staff',
    Staff.fromJson,
    {'page': page, 'limit': limit},
  );

  /// `POST /api/admin/staff` (`role` `owner`/`admin`/`staff`); [Staff.tempPassword] ekbar; duplicate `409 staff_exists`.
  Future<Staff> create(String email, String name, String role) async =>
      Staff.fromJson(
        await _json(api, 'POST', '/api/admin/staff', {
          'email': email,
          'name': name,
          'role': role,
        }),
      );

  /// `PATCH /api/admin/staff/{id}` (shudhu deya field).
  Future<Staff> update(
    int id, {
    String? name,
    String? role,
    bool? isActive,
  }) async => Staff.fromJson(
    await _json(api, 'PATCH', '/api/admin/staff/$id', {
      'name': ?name,
      'role': ?role,
      'is_active': ?isActive,
    }),
  );

  /// `POST /api/admin/staff/{id}/reset` → notun temporary password (ekbar).
  Future<String> reset(int id) async => field(
    await _json(api, 'POST', '/api/admin/staff/$id/reset'),
    'temp_password',
  );
}

/// Audit log (owner/admin, read-only).
class AuditRepository {
  /// Client.
  final ApiClient api;

  /// AuditRepository.
  const AuditRepository(this.api);

  /// `GET /api/admin/audit-logs` (`from`/`to` din, `to` shoho; notun age).
  Future<Page<AuditEntry>> logs({
    String? action,
    int? staffId,
    String? from,
    String? to,
    int? page,
    int? limit,
  }) => _page(api, '/api/admin/audit-logs', AuditEntry.fromJson, {
    'action': action,
    'staff_id': staffId,
    'from': from,
    'to': to,
    'page': page,
    'limit': limit,
  });
}

/// Dashboard (shob role; vendor nijer).
class DashboardRepository {
  /// Client.
  final ApiClient api;

  /// DashboardRepository.
  const DashboardRepository(this.api);

  /// `GET /api/admin/dashboard`.
  Future<Dashboard> dashboard() async =>
      Dashboard.fromJson(await _json(api, 'GET', '/api/admin/dashboard'));
}
