import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';
import 'package:test/test.dart';

// Path/method/body/query Go er router er moto (backend/cmd/api/router.go, */admin.go; 7.7, 11.7).
void main() {
  late List<http.BaseRequest> seen;
  late List<List<int>> bodies;
  ApiClient api(Object? Function(http.BaseRequest) body, {int status = 200}) {
    seen = [];
    bodies = [];
    return ApiClient(
      Uri.parse('http://api.test'),
      MockClient((r) async {
        seen.add(r);
        bodies.add(r.bodyBytes);
        final b = body(r);
        return b == null
            ? http.Response('', 204)
            : http.Response(
                jsonEncode(b),
                status,
                headers: {'content-type': 'application/json; charset=utf-8'},
              );
      }),
    );
  }

  String call() =>
      '${seen.single.method} ${seen.single.url.path}${seen.single.url.hasQuery ? '?${seen.single.url.query}' : ''}';
  Object? sent() => jsonDecode(utf8.decode(bodies.single));
  Map<String, Object> paged(List<Object> items) => {
    'items': items,
    'page': 1,
    'limit': 24,
    'total': items.length,
  };

  final orderJson = {
    'id': 7,
    'order_number': 'SB-100001',
    'status': 'pending',
    'display_status': 'pending',
    'recipient_name': 'রহিম',
    'recipient_phone': '+8801712345678',
    'district': 'ঢাকা',
    'area': 'ধানমন্ডি',
    'total': 96000,
    'placed_at': '2026-11-16T04:30:00Z',
    'delivery_deadline': null,
  };
  final staffJson = {
    'id': 2,
    'email': 's@example.com',
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

  group('AdminAuthRepository (7.7, 8.3)', () {
    test('login: TOTP lage ba setup', () async {
      final r = await AdminAuthRepository(
        api((_) => {'totp_required': true, 'temp_token': 'TT'}),
      ).login('sobujbazar', 'o@example.com', 'secret-password');
      expect(call(), 'POST /api/admin/auth/login');
      expect(sent(), {
        'shop': 'sobujbazar',
        'email': 'o@example.com',
        'password': 'secret-password',
      });
      expect(
        [r.totpRequired, r.tempToken, r.setupRequired],
        [true, 'TT', false],
      );
      final s = await AdminAuthRepository(
        api((_) => {'setup_required': true}),
      ).login('s', 'e', 'p');
      expect(
        [s.totpRequired, s.tempToken, s.setupRequired],
        [false, null, true],
      );
    });
    test('totp, recovery → setup_required', () async {
      expect(
        await AdminAuthRepository(
          api((_) => {'setup_required': false}),
        ).totp('TT', '123456'),
        isFalse,
      );
      expect(call(), 'POST /api/admin/auth/totp');
      expect(sent(), {'temp_token': 'TT', 'code': '123456'});
      expect(
        await AdminAuthRepository(
          api((_) => {'setup_required': true}),
        ).recovery('TT', 'ABCD-EFGH'),
        isTrue,
      );
      expect(call(), 'POST /api/admin/auth/recovery');
    });
    test('password, totp setup/enable, logout, me', () async {
      await AdminAuthRepository(
        api((_) => null),
      ).changePassword('old-password', 'new-password!');
      expect(call(), 'POST /api/admin/auth/password');
      expect(sent(), {'old': 'old-password', 'new': 'new-password!'});
      final t = await AdminAuthRepository(
        api((_) => {'otpauth_url': 'otpauth://totp/x', 'secret': 'ABC'}),
      ).totpSetup();
      expect(call(), 'POST /api/admin/auth/totp/setup');
      expect([t.otpauthUrl, t.secret], ['otpauth://totp/x', 'ABC']);
      final codes = await AdminAuthRepository(
        api(
          (_) => {
            'recovery_codes': ['A', 'B'],
          },
        ),
      ).totpEnable('123456');
      expect(call(), 'POST /api/admin/auth/totp/enable');
      expect(sent(), {'code': '123456'});
      expect(codes, ['A', 'B']);
      await AdminAuthRepository(api((_) => null)).logout();
      expect(call(), 'POST /api/admin/auth/logout');
      final me = await AdminAuthRepository(
        api(
          (_) => {
            'name': 'মালিক',
            'email': 'o@example.com',
            'role': 'owner',
            'vendor_id': null,
          },
        ),
      ).me();
      expect(call(), 'GET /api/admin/me');
      expect(me.role, 'owner');
    });
  });

  group('AdminOrderRepository (7.7, 8.8, 8.11, 11.7)', () {
    test('orders: filter query (na deya na), page', () async {
      final p = await AdminOrderRepository(api((_) => paged([orderJson])))
          .orders(
            status: 'pending',
            tab: 'awaiting_confirm',
            risk: true,
            q: '01712345678',
            from: '2026-11-01',
            to: '2026-11-17',
            vendorId: 2,
            page: 2,
            limit: 50,
          );
      expect(seen.single.url.path, '/api/admin/orders');
      expect(seen.single.url.queryParameters, {
        'status': 'pending',
        'tab': 'awaiting_confirm',
        'risk': '1',
        'q': '01712345678',
        'from': '2026-11-01',
        'to': '2026-11-17',
        'vendor_id': '2',
        'page': '2',
        'limit': '50',
      });
      expect(p.items.single.orderNumber, 'SB-100001');
      await AdminOrderRepository(api((_) => paged([]))).orders();
      expect(call(), 'GET /api/admin/orders');
    });
    test('order detail', () async {
      final d = await AdminOrderRepository(
        api(
          (_) => {
            ...orderJson,
            'address': 'x',
            'note': null,
            'cancel_reason': null,
            'fulfillments': <Object>[],
            'history': <Object>[],
          },
        ),
      ).order(7);
      expect(call(), 'GET /api/admin/orders/7');
      expect(d.id, 7);
    });
    test('transition, note, edit', () async {
      final t = await AdminOrderRepository(
        api((_) => {'status': 'confirmed', 'display_status': 'confirmed'}),
      ).transition(7, 'confirmed');
      expect(call(), 'POST /api/admin/orders/7/transition');
      expect(sent(), {'to': 'confirmed'});
      expect([t.status, t.displayStatus], ['confirmed', 'confirmed']);
      await AdminOrderRepository(
        api((_) => {'status': 'cancelled', 'display_status': 'cancelled'}),
      ).transition(7, 'cancelled', note: 'customer chay na');
      expect(sent(), {'to': 'cancelled', 'note': 'customer chay na'});
      final n = await AdminOrderRepository(
        api(
          (_) => {
            'id': 1,
            'staff_id': 2,
            'kind': 'call',
            'note': 'ok',
            'created_at': '2026-11-16T05:00:00Z',
          },
        ),
      ).addNote(7, 'ok', kind: 'call');
      expect(call(), 'POST /api/admin/orders/7/notes');
      expect(sent(), {'kind': 'call', 'note': 'ok'});
      expect(n.id, 1);
      final e = await AdminOrderRepository(
        api((_) => {'status': 'confirmed', 'display_status': 'confirmed'}),
      ).editItems(7, {11: 1, 12: 0}, 'customer bollo');
      expect(call(), 'PATCH /api/admin/orders/7/items');
      expect(sent(), {
        'items': [
          {'item_id': 11, 'quantity': 1},
          {'item_id': 12, 'quantity': 0},
        ],
        'note': 'customer bollo',
      });
      expect(e.status, 'confirmed');
    });
    test('fulfillments, transition (return restock), pack', () async {
      await AdminOrderRepository(
        api((_) => paged([])),
      ).fulfillments(status: 'open', date: '2026-11-17');
      expect(seen.single.url.path, '/api/admin/fulfillments');
      expect(seen.single.url.queryParameters, {
        'status': 'open',
        'date': '2026-11-17',
      });
      expect(
        await AdminOrderRepository(
          api((_) => {'status': 'returned'}),
        ).transitionFulfillment(
          5,
          'returned',
          note: 'pocha',
          restockItemIds: [11],
        ),
        'returned',
      );
      expect(call(), 'POST /api/admin/fulfillments/5/transition');
      expect(sent(), {
        'to': 'returned',
        'note': 'pocha',
        'restock_item_ids': [11],
      });
      final p = await AdminOrderRepository(
        api((_) => {'status': 'packed', 'display_status': 'packed'}),
      ).pack(5, {12: 1040}, customerOkNote: 'phone e raji');
      expect(call(), 'POST /api/admin/fulfillments/5/pack');
      expect(sent(), {
        'items': [
          {'item_id': 12, 'packed_weight_g': 1040},
        ],
        'customer_ok_note': 'phone e raji',
      });
      expect(p.displayStatus, 'packed');
      await AdminOrderRepository(
        api((_) => {'status': 'packed', 'display_status': 'packed'}),
      ).pack(5, {});
      expect(sent(), {'items': <Object>[]});
    });
    test('ship, shipment update', () async {
      final s = await AdminOrderRepository(
        api(
          (_) => {
            'id': 9,
            'fulfillment_id': 5,
            'carrier': 'own_rider',
            'status': 'booked',
            'cod_amount': 96000,
            'display_status': 'shipped',
          },
        ),
      ).ship(5, 'own_rider', riderStaffId: 3);
      expect(call(), 'POST /api/admin/fulfillments/5/shipments');
      expect(sent(), {'carrier': 'own_rider', 'rider_staff_id': 3});
      expect(s.codAmount, 96000);
      await AdminOrderRepository(
        api(
          (_) => {
            'id': 9,
            'status': 'delivered',
            'display_status': 'delivered',
          },
        ),
      ).updateShipment(9, status: 'delivered');
      expect(call(), 'PATCH /api/admin/shipments/9');
      expect(sent(), {'status': 'delivered'});
    });
    test('pick list, print link', () async {
      await AdminOrderRepository(
        api(
          (_) => {
            'date': '2026-11-17',
            'variants': <Object>[],
            'areas': <Object>[],
            'fulfillments': <Object>[],
          },
        ),
      ).pickList(date: '2026-11-17');
      expect(call(), 'GET /api/admin/packing/pick-list?date=2026-11-17');
      final l = await AdminOrderRepository(
        api(
          (_) => {
            'url': 'https://sobujbazar.com/print/invoice?token=T',
            'expires_at': '2026-11-17T05:10:00Z',
          },
        ),
      ).printLink('invoice', orderIds: [7, 8]);
      expect(call(), 'POST /api/admin/print-links');
      expect(sent(), {
        'kind': 'invoice',
        'order_ids': [7, 8],
      });
      expect(l.url, startsWith('https://'));
      expect(l.expiresAt, DateTime.utc(2026, 11, 17, 5, 10));
      await AdminOrderRepository(
        api((_) => {'url': 'u', 'expires_at': '2026-11-17T05:10:00Z'}),
      ).printLink('pick_list');
      expect(sent(), {'kind': 'pick_list'});
    });
  });

  group('InventoryRepository (8.12, 11.7 item 1, 2)', () {
    test('variants, batches, movements', () async {
      await InventoryRepository(
        api((_) => paged([])),
      ).variants(q: 'চাল', low: true);
      expect(seen.single.url.path, '/api/admin/inventory/variants');
      expect(seen.single.url.queryParameters, {'q': 'চাল', 'low': '1'});
      await InventoryRepository(api((_) => paged([]))).batches(3);
      expect(call(), 'GET /api/admin/variants/3/batches');
      await InventoryRepository(
        api((_) => paged([])),
      ).movements(variantId: 3, page: 2);
      expect(call(), 'GET /api/admin/inventory/movements?variant_id=3&page=2');
    });
    test('batch create, adjust', () async {
      final b = await InventoryRepository(
        api(
          (_) => {
            'id': 1,
            'variant_id': 3,
            'batch_code': null,
            'quantity_received': 10,
            'quantity_remaining': 10,
            'harvest_date': null,
            'expiry_date': '2026-12-01',
            'received_at': '2026-11-17T04:00:00Z',
          },
        ),
      ).createBatch(3, 10, expiryDate: '2026-12-01');
      expect(call(), 'POST /api/admin/batches');
      expect(sent(), {
        'variant_id': 3,
        'quantity': 10,
        'expiry_date': '2026-12-01',
      });
      expect(b.quantityRemaining, 10);
      final stock = await InventoryRepository(
        api((_) => {'variant_id': 3, 'stock': 8}),
      ).adjust(3, -2, 'wastage', 'pocha', batchId: 1);
      expect(call(), 'POST /api/admin/inventory/adjust');
      expect(sent(), {
        'variant_id': 3,
        'change': -2,
        'reason': 'wastage',
        'note': 'pocha',
        'batch_id': 1,
      });
      expect(stock, 8);
    });
  });

  group('AdminCatalogRepository: chobi (11.7 item 9)', () {
    test('upload multipart: file, alt_bn, alt_en', () async {
      final i = await AdminCatalogRepository(
        api(
          (_) => {
            'id': 1,
            'product_id': 2,
            'storage_key': 'shops/1/products/2/A.jpg',
            'alt': {'bn': 'চাল'},
            'width': 800,
            'height': 600,
            'sort_order': 0,
          },
          status: 201,
        ),
      ).uploadImage(2, [0xff, 0xd8, 0xff], 'rice.jpg', altBn: 'চাল');
      expect(call(), 'POST /api/admin/products/2/images');
      expect(
        seen.single.headers['content-type'],
        startsWith('multipart/form-data; boundary='),
      );
      final form = latin1.decode(bodies.single); // jpeg byte utf8 na
      expect(form, contains('name="file"; filename="rice.jpg"'));
      expect(form, contains('name="alt_bn"'));
      expect(form, isNot(contains('alt_en')));
      expect(i.storageKey, 'shops/1/products/2/A.jpg');
    });
    test('order, delete', () async {
      await AdminCatalogRepository(
        api(
          (_) => {
            'ids': [3, 1],
          },
        ),
      ).orderImages(2, [3, 1]);
      expect(call(), 'PATCH /api/admin/products/2/images/order');
      expect(sent(), {
        'ids': [3, 1],
      });
      await AdminCatalogRepository(api((_) => null)).deleteImage(3);
      expect(call(), 'DELETE /api/admin/images/3');
    });
  });

  group('AdminCatalogRepository: product, variant, category (7.7)', () {
    final cat = {
      'id': 4,
      'parent_id': null,
      'slug': 'rice',
      'name': {'bn': 'চাল'},
      'image_key': null,
      'sort_order': 0,
      'is_active': true,
    };
    final product = {
      'id': 2,
      'vendor_id': 1,
      'category_id': null,
      'slug': 'red-rice',
      'name': {'bn': 'লাল চাল'},
      'description': <String, Object>{},
      'unit': 'piece',
      'variable_weight': false,
      'search_keywords': '',
      'is_active': true,
    };
    final variant = {
      'id': 3,
      'product_id': 2,
      'sku': 'RICE-1',
      'options': <String, Object>{},
      'price': 12000,
      'compare_at_price': null,
      'cost_price': null,
      'nominal_weight_g': null,
      'stock': 0,
      'low_stock_at': 5,
      'is_active': true,
      'sort_order': 0,
    };
    test('category: list (array, paging na), create, update, delete', () async {
      final l = await AdminCatalogRepository(
        api((_) => [cat]),
      ).categories();
      expect(call(), 'GET /api/admin/categories');
      expect(l.single.slug, 'rice');
      final body = {
        'slug': 'rice',
        'name': {'bn': 'চাল', 'en': 'Rice'},
        'parent_id': null,
      };
      await AdminCatalogRepository(
        api((_) => cat, status: 201),
      ).createCategory(body);
      expect(call(), 'POST /api/admin/categories');
      expect(sent(), body);
      await AdminCatalogRepository(
        api((_) => cat),
      ).updateCategory(4, {'is_active': false});
      expect(call(), 'PATCH /api/admin/categories/4');
      expect(sent(), {'is_active': false});
      await AdminCatalogRepository(api((_) => null)).deleteCategory(4);
      expect(call(), 'DELETE /api/admin/categories/4');
    });
    test('product: list page, detail, create, update', () async {
      final p = await AdminCatalogRepository(
        api((_) => paged([product])),
      ).products(page: 2);
      expect(call(), 'GET /api/admin/products?page=2');
      expect(p.items.single.slug, 'red-rice');
      final d = await AdminCatalogRepository(
        api(
          (_) => {
            ...product,
            'variants': [variant],
            'images': <Object>[],
          },
        ),
      ).product(2);
      expect(call(), 'GET /api/admin/products/2');
      expect(d.variants.single.price, 12000);
      final body = {
        'slug': 'red-rice',
        'name': {'bn': 'লাল চাল'},
      };
      await AdminCatalogRepository(
        api((_) => product, status: 201),
      ).createProduct(body);
      expect(call(), 'POST /api/admin/products');
      expect(sent(), body);
      await AdminCatalogRepository(
        api((_) => product),
      ).updateProduct(2, {'is_active': false});
      expect(call(), 'PATCH /api/admin/products/2');
      expect(sent(), {'is_active': false});
    });
    test('variant: create, update', () async {
      final body = {'sku': 'RICE-1', 'price': 12000};
      final v = await AdminCatalogRepository(
        api((_) => variant, status: 201),
      ).createVariant(2, body);
      expect(call(), 'POST /api/admin/products/2/variants');
      expect(sent(), body);
      expect(v.sku, 'RICE-1');
      await AdminCatalogRepository(
        api((_) => variant),
      ).updateVariant(3, {'compare_at_price': null});
      expect(call(), 'PATCH /api/admin/variants/3');
      expect(sent(), {'compare_at_price': null});
    });
    test('409 sku_taken → ApiError', () async {
      final e =
          await AdminCatalogRepository(
                api(
                  (_) => {
                    'error': {'code': 'sku_taken', 'message': 'x'},
                  },
                  status: 409,
                ),
              )
              .createVariant(2, {'sku': 'X', 'price': 1})
              .then<ApiError?>((_) => null, onError: (Object e) => e as ApiError);
      expect(e!.code, 'sku_taken');
    });
  });

  group('Zone, coupon (11.7 item 13 v, vi)', () {
    final zone = {
      'id': 2,
      'name': {'bn': 'ঢাকা'},
      'district': 'ঢাকা',
      'area': null,
      'charge': 6000,
      'min_order': 0,
      'free_delivery_over': null,
      'same_city': true,
      'is_active': true,
    };
    final coupon = {
      'id': 1,
      'code': 'EID10',
      'kind': 'fixed',
      'value': 5000,
      'min_order': 0,
      'max_discount': null,
      'usage_limit': null,
      'used_count': 0,
      'per_phone_limit': 1,
      'starts_at': null,
      'expires_at': null,
      'is_active': true,
      'created_at': '2026-11-16T04:30:00Z',
    };
    test('zone list, create, update', () async {
      await AdminZoneRepository(api((_) => paged([zone]))).zones(page: 1);
      expect(call(), 'GET /api/admin/delivery-zones?page=1');
      final body = {
        'name': {'bn': 'ঢাকা'},
        'district': 'ঢাকা',
        'charge': 6000,
        'same_city': true,
      };
      await AdminZoneRepository(api((_) => zone, status: 201)).create(body);
      expect(call(), 'POST /api/admin/delivery-zones');
      expect(sent(), body);
      await AdminZoneRepository(
        api((_) => zone),
      ).update(2, {'is_active': false});
      expect(call(), 'PATCH /api/admin/delivery-zones/2');
      expect(sent(), {'is_active': false});
    });
    test('coupon list, create, update', () async {
      final p = await AdminCouponRepository(
        api((_) => paged([coupon])),
      ).coupons();
      expect(call(), 'GET /api/admin/coupons');
      expect(p.items.single.code, 'EID10');
      await AdminCouponRepository(
        api((_) => coupon, status: 201),
      ).create({'code': 'eid10', 'kind': 'fixed', 'value': 5000});
      expect(call(), 'POST /api/admin/coupons');
      await AdminCouponRepository(
        api((_) => coupon),
      ).update(1, {'is_active': false});
      expect(call(), 'PATCH /api/admin/coupons/1');
    });
    test('409 coupon_exists → ApiError', () async {
      final e =
          await AdminCouponRepository(
                api(
                  (_) => {
                    'error': {'code': 'coupon_exists', 'message': 'x'},
                  },
                  status: 409,
                ),
              )
              .create({'code': 'EID10'})
              .then<ApiError?>(
                (_) => null,
                onError: (Object e) => e as ApiError,
              );
      expect([e!.status, e.code], [409, 'coupon_exists']);
    });
  });

  group('AdminCustomerRepository (11.7 item 12)', () {
    test(
      'customers, customer, blocked, block, unblock (phone body te, URL e na)',
      () async {
        await AdminCustomerRepository(
          api((_) => paged([])),
        ).customers(q: 'রহিম');
        expect(seen.single.url.path, '/api/admin/customers');
        expect(seen.single.url.queryParameters, {'q': 'রহিম'});
        await AdminCustomerRepository(
          api(
            (_) => {
              'id': 1,
              'phone': '+8801712345678',
              'name': null,
              'email': null,
              'locale': 'bn',
              'created_at': '2026-11-01T00:00:00Z',
              'blocked': false,
              'orders': <Object>[],
            },
          ),
        ).customer(1);
        expect(call(), 'GET /api/admin/customers/1');
        await AdminCustomerRepository(api((_) => paged([]))).blockedPhones();
        expect(call(), 'GET /api/admin/blocked-phones');
        final row = {
          'phone': '+8801712345678',
          'reason': 'fake',
          'staff_id': 1,
          'created_at': '2026-11-16T04:30:00Z',
        };
        final b = await AdminCustomerRepository(
          api((_) => row, status: 201),
        ).block('01712345678', reason: 'fake');
        expect(call(), 'POST /api/admin/blocked-phones');
        expect(sent(), {'phone': '01712345678', 'reason': 'fake'});
        expect(b.reason, 'fake');
        await AdminCustomerRepository(api((_) => null)).unblock('01712345678');
        expect(call(), 'DELETE /api/admin/blocked-phones');
        expect(sent(), {'phone': '01712345678'});
      },
    );
  });

  group('StaffRepository (11.7 item 11)', () {
    test('list, create (temp_password), update, reset', () async {
      await StaffRepository(api((_) => paged([staffJson]))).staff();
      expect(call(), 'GET /api/admin/staff');
      final s = await StaffRepository(
        api(
          (_) => {...staffJson, 'temp_password': 'ABCDEFGHIJKLMNOPQRSTUVWXYZ'},
          status: 201,
        ),
      ).create('s@example.com', 'করিম', 'staff');
      expect(call(), 'POST /api/admin/staff');
      expect(sent(), {
        'email': 's@example.com',
        'name': 'করিম',
        'role': 'staff',
      });
      expect(s.tempPassword, 'ABCDEFGHIJKLMNOPQRSTUVWXYZ');
      await StaffRepository(api((_) => staffJson)).update(2, isActive: false);
      expect(call(), 'PATCH /api/admin/staff/2');
      expect(sent(), {'is_active': false});
      expect(
        await StaffRepository(api((_) => {'temp_password': 'ZYX'})).reset(2),
        'ZYX',
      );
      expect(call(), 'POST /api/admin/staff/2/reset');
    });
  });

  group('Audit, dashboard (11.7 item 11, 13 viii)', () {
    test('audit logs filter', () async {
      await AuditRepository(api((_) => paged([]))).logs(
        action: 'staff.reset',
        staffId: 2,
        from: '2026-11-01',
        to: '2026-11-17',
      );
      expect(seen.single.url.path, '/api/admin/audit-logs');
      expect(seen.single.url.queryParameters, {
        'action': 'staff.reset',
        'staff_id': '2',
        'from': '2026-11-01',
        'to': '2026-11-17',
      });
    });
    test('dashboard', () async {
      final d = await DashboardRepository(
        api(
          (_) => {
            'date': '2026-11-17',
            'orders': 1,
            'sales': 2,
            'pending': 3,
            'to_pack': 4,
            'low_stock': 5,
            'expiring': 6,
            'deadline_soon': 7,
          },
        ),
      ).dashboard();
      expect(call(), 'GET /api/admin/dashboard');
      expect(d.deadlineSoon, 7);
    });
  });
}
