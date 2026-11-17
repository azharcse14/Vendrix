import 'package:admin/features/customers/bloc/customer_detail_cubit.dart';
import 'package:admin/features/customers/bloc/customers_cubit.dart';
import 'package:admin/features/customers/pages/customer_detail_page.dart';
import 'package:admin/features/customers/pages/customers_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> customerJson(
  int id, {
  bool blocked = false,
  bool detail = false,
}) => {
  'id': id,
  'phone': '+8801711000001',
  'name': 'Karim $id',
  'email': null,
  'locale': 'bn',
  'created_at': '2026-11-01T04:00:00Z',
  'blocked': blocked,
  if (!detail) 'order_count': 3,
  if (!detail) 'order_total': 250000,
  if (detail)
    'orders': [
      {
        'id': 7,
        'order_number': 'SB-7',
        'display_status': 'delivered',
        'payment_method': 'cod',
        'payment_status': 'paid',
        'total': 125000,
        'placed_at': '2026-11-10T04:00:00Z',
      },
    ],
};

Map<String, Object?> blockedJson(String phone) => {
  'phone': phone,
  'reason': 'fake order',
  'staff_id': 1,
  'created_at': '2026-11-12T04:00:00Z',
};

Future<FakeApi> pumpList(
  WidgetTester tester, {
  Map<String, List<dynamic>> extra = const {},
  Size size = const Size(1280, 900),
  String locale = 'bn',
}) async {
  final api = FakeApi({
    'GET /api/admin/customers': [
      json(200, page([customerJson(1)])),
    ],
    'GET /api/admin/blocked-phones': [
      json(200, page([blockedJson('+8801811000002')])),
    ],
    for (final e in extra.entries) e.key: [...e.value.cast()],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => CustomersCubit(api.customers)..load(),
      child: const CustomersPage(),
    ),
    size: size,
    locale: locale,
  );
  return api;
}

Future<FakeApi> pumpDetail(
  WidgetTester tester, {
  bool blocked = false,
  Map<String, List<dynamic>> extra = const {},
  Size size = const Size(1280, 900),
}) async {
  final api = FakeApi({
    'GET /api/admin/customers/1': [
      json(200, customerJson(1, blocked: blocked, detail: true)),
      json(200, customerJson(1, blocked: !blocked, detail: true)),
    ],
    for (final e in extra.entries) e.key: [...e.value.cast()],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => CustomerDetailCubit(api.customers, 1)..load(),
      child: const CustomerDetailPage(),
    ),
    size: size,
  );
  return api;
}

void main() {
  for (final w in [360.0, 1280.0]) {
    testWidgets('list ${w.toInt()}px: overflow nai', (tester) async {
      await pumpList(tester, size: Size(w, 800));
      expect(tester.takeException(), isNull);
      expect(find.byKey(const Key('customer.1')), findsOneWidget);
    });
  }

  testWidgets('khoja: q trim kore GET', (tester) async {
    final api = await pumpList(tester);
    await tester.enterText(
      find.byKey(const Key('customers.search')),
      ' 01711 ',
    );
    await tester.testTextInput.receiveAction(TextInputAction.search);
    await tester.pumpAndSettle();
    expect(api.calls.last, 'GET /api/admin/customers?q=01711');
  });

  testWidgets('blocked list, notun block (reason), unblock confirm er por', (
    tester,
  ) async {
    final api = await pumpList(
      tester,
      extra: {
        'POST /api/admin/blocked-phones': [
          json(201, blockedJson('+8801911000003')),
        ],
        'DELETE /api/admin/blocked-phones': [json(204, null)],
      },
    );
    await tester.tap(find.byKey(const Key('customers.blocked')));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'GET /api/admin/blocked-phones');
    expect(find.text('+8801811000002'), findsOneWidget);

    await tester.tap(find.byKey(const Key('blocked.new')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('block.phone')), '123');
    await tester.tap(find.byKey(const Key('block.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/blocked-phones'], isNull); // bhul phone
    await tester.enterText(
      find.byKey(const Key('block.phone')),
      '01911-000003',
    );
    await tester.enterText(find.byKey(const Key('block.reason')), ' fake ');
    await tester.tap(find.byKey(const Key('block.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/blocked-phones'], {
      'phone': '+8801911000003',
      'reason': 'fake',
    });

    await tester.tap(find.byKey(const Key('blocked.+8801811000002.unblock')));
    await tester.pumpAndSettle();
    expect(api.calls, isNot(contains('DELETE /api/admin/blocked-phones')));
    await tester.tap(find.byKey(const Key('confirm.ok')));
    await tester.pumpAndSettle();
    expect(api.bodies['DELETE /api/admin/blocked-phones'], {
      'phone': '+8801811000002',
    });
  });

  for (final w in [360.0, 1280.0]) {
    testWidgets('detail ${w.toInt()}px: shesh order, overflow nai', (
      tester,
    ) async {
      await pumpDetail(tester, size: Size(w, 800));
      expect(tester.takeException(), isNull);
      expect(find.text('SB-7'), findsOneWidget);
      expect(find.byKey(const Key('customer.block')), findsOneWidget);
      expect(find.byKey(const Key('customer.unblock')), findsNothing);
    });
  }

  testWidgets('detail block: reason optional, tarpor abar load', (
    tester,
  ) async {
    final api = await pumpDetail(
      tester,
      extra: {
        'POST /api/admin/blocked-phones': [
          json(201, blockedJson('+8801711000001')),
        ],
      },
    );
    await tester.tap(find.byKey(const Key('customer.block')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('note.submit')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/blocked-phones'], {
      'phone': '+8801711000001',
    });
    expect(find.byKey(const Key('customer.unblock')), findsOneWidget);
  });

  testWidgets('detail unblock: confirm er por DELETE', (tester) async {
    final api = await pumpDetail(
      tester,
      blocked: true,
      extra: {
        'DELETE /api/admin/blocked-phones': [json(204, null)],
      },
    );
    await tester.tap(find.byKey(const Key('customer.unblock')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('confirm.ok')));
    await tester.pumpAndSettle();
    expect(api.bodies['DELETE /api/admin/blocked-phones'], {
      'phone': '+8801711000001',
    });
    expect(find.byKey(const Key('customer.block')), findsOneWidget);
  });
}
