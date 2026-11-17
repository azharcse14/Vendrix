import 'dart:convert';

import 'package:admin/core/auth/auth_bloc.dart';
import 'package:admin/core/browser.dart';
import 'package:admin/core/l10n/locale_cubit.dart';
import 'package:admin/core/theme/theme_cubit.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';

http.Response json(int status, Object? body) => http.Response(
  body == null ? '' : jsonEncode(body),
  status,
  headers: {'content-type': 'application/json; charset=utf-8'},
);

http.Response err(
  int status,
  String code, [
  Map<String, String>? fields,
  Map<String, Object?>? details,
]) => json(status, {
  'error': {
    'code': code,
    'message': code,
    'fields': ?fields,
    'details': ?details,
  },
});

/// Fake API: "METHOD path" → response er queue (shesh ta bar bar); [calls] e "METHOD path?query", [bodies] e body.
class FakeApi {
  final Map<String, List<http.Response>> routes;
  final calls = <String>[];
  final bodies = <String, Object?>{};
  FakeApi(this.routes);

  late final client = ApiClient(
    Uri.parse('https://api.test'),
    MockClient((req) async {
      final key = '${req.method} ${req.url.path}';
      calls.add(req.url.hasQuery ? '$key?${req.url.query}' : key);
      if (req.headers['content-type']?.startsWith('application/json') ??
          false) {
        bodies[key] = jsonDecode(req.body);
      } else if (req.bodyBytes.isNotEmpty) {
        bodies[key] = req.bodyBytes.length; // multipart: shudhu size
      }
      final q = routes[key];
      if (q == null || q.isEmpty) return err(404, 'not_found');
      return q.length == 1 ? q.first : q.removeAt(0);
    }),
  );

  AdminOrderRepository get orders => AdminOrderRepository(client);
  AdminCatalogRepository get catalog => AdminCatalogRepository(client);
  InventoryRepository get inventory => InventoryRepository(client);
  StaffRepository get staff => StaffRepository(client);
}

StaffProfile profile(String role) => StaffProfile.fromJson({
  'name': 'Rahim',
  'email': 'n@example.com',
  'role': role,
  'vendor_id': role.startsWith('vendor') ? 2 : null,
});

/// Page er JSON (7.1).
Map<String, Object?> page(
  List<Object?> items, {
  int page = 1,
  int total = -1,
}) => {
  'items': items,
  'page': page,
  'limit': 20,
  'total': total < 0 ? items.length : total,
};

Map<String, Object?> orderJson(
  int id, {
  String status = 'pending',
  String? display,
  String method = 'cod',
  String payment = 'unpaid',
  List<String> risk = const [],
  List<Map<String, Object?>> fulfillments = const [],
}) => {
  'id': id,
  'order_number': 'SB-$id',
  'status': status,
  'display_status': display ?? status,
  'recipient_name': 'Karim Uddin',
  'recipient_phone': '+8801711000001',
  'district': 'Dhaka',
  'area': 'Mirpur',
  'total': 125000,
  'placed_at': '2026-11-17T04:00:00Z',
  'delivery_deadline': null,
  'customer_name': 'Karim Uddin',
  'customer_phone': '+8801711000001',
  'payment_method': method,
  'payment_status': payment,
  'risk_flags': risk,
  'subtotal': 120000,
  'discount': 0,
  'delivery_charge': 5000,
  'address': 'House 1, Road 2',
  'note': null,
  'cancel_reason': null,
  'fulfillments': fulfillments,
  'history': [],
  'notes': [],
};

Map<String, Object?> itemJson(int id, {int fid = 10, bool variable = false}) =>
    {
      'id': id,
      'fulfillment_id': fid,
      'variant_id': 100 + id,
      'product_name': {'bn': 'Chal', 'en': 'Rice'},
      'variant_label': {'bn': '1 kg', 'en': '1 kg'},
      'sku': 'SKU-$id',
      'unit_price': 10000,
      'quantity': 2,
      'variable_weight': variable,
      'ordered_weight_g': variable ? 1000 : null,
      'packed_weight_g': null,
      'line_total': 20000,
      'discount_share': 0,
      'status': 'active',
    };

Map<String, Object?> fulfillmentJson(
  int id, {
  String status = 'open',
  List<Map<String, Object?>>? items,
}) => {
  'id': id,
  'vendor_id': 1,
  'seq': 1,
  'status': status,
  'subtotal': 120000,
  'discount': 0,
  'delivery_charge': 5000,
  'total': 125000,
  'items': items ?? [itemJson(1, fid: id)],
};

/// [pumpPage] er [Browser]: notun tab er url ar shoja khola url (`tel:`).
final openedTabs = <Future<String>>[];
final openedUrls = <String>[];

/// [pumpPage] er [Browser.pickFile] er uttor (null = bondho korlo).
PickedFile? pickedFile;

/// Page ke app er bloc (auth, theme, bhasha) shoho; [size] logical pixel (360 = phone).
Future<void> pumpPage(
  WidgetTester tester,
  Widget child, {
  String role = 'owner',
  Size size = const Size(1280, 900),
  String locale = 'bn',
}) async {
  tester.view.physicalSize = size;
  tester.view.devicePixelRatio = 1;
  addTearDown(tester.view.reset);
  final auth = _FixedAuth(Authenticated(profile(role)));
  final lc = LocaleCubit();
  if (locale != 'bn') lc.toggle();
  openedTabs.clear();
  openedUrls.clear();
  await tester.pumpWidget(
    RepositoryProvider.value(
      value: Browser(
        openTab: openedTabs.add,
        open: openedUrls.add,
        pickFile: () async => pickedFile,
      ),
      child: MultiBlocProvider(
        providers: [
          BlocProvider<AuthBloc>.value(value: auth),
          BlocProvider(create: (_) => ThemeCubit()),
          BlocProvider.value(value: lc),
        ],
        child: Builder(
          builder: (context) => MaterialApp(
            theme: context.watch<ThemeCubit>().state,
            home: child,
          ),
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

// Login er state bodlay na (page test e network chhara).
class _FixedAuth extends AuthBloc {
  _FixedAuth(AuthState s)
    : super(
        AdminAuthRepository(
          ApiClient(
            Uri.parse('https://x'),
            MockClient((_) async => http.Response('', 500)),
          ),
        ),
      ) {
    emit(s);
  }
}
