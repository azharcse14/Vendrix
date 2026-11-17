import 'package:admin/features/orders/bloc/order_detail_cubit.dart';
import 'package:admin/features/orders/pages/order_detail_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

void main() {
  final opened = <Future<String>>[];
  setUp(opened.clear);

  OrderDetailCubit cubit(FakeApi api) =>
      OrderDetailCubit(api.orders, opened.add, 1)..load();

  Future<OrderDetailLoaded> loaded(OrderDetailCubit c) async =>
      (c.state is OrderDetailLoaded && !(c.state as OrderDetailLoaded).busy
              ? c.state
              : await c.stream.firstWhere(
                  (s) => s is OrderDetailLoaded && !s.busy,
                ))
          as OrderDetailLoaded;

  test('confirm → transition POST, tarpor abar load', () async {
    final api = FakeApi({
      'GET /api/admin/orders/1': [
        json(200, orderJson(1)),
        json(200, orderJson(1, status: 'confirmed')),
      ],
      'POST /api/admin/orders/1/transition': [
        json(200, {'status': 'confirmed', 'display_status': 'confirmed'}),
      ],
    });
    final c = cubit(api);
    await loaded(c);
    await c.transition('confirmed');
    expect(api.bodies['POST /api/admin/orders/1/transition'], {
      'to': 'confirmed',
    });
    expect((await loaded(c)).order.status, 'confirmed');
  });

  test('bhul (payment_required) state e error, order thake', () async {
    final api = FakeApi({
      'GET /api/admin/orders/1': [json(200, orderJson(1))],
      'POST /api/admin/orders/1/transition': [err(409, 'payment_required')],
    });
    final c = cubit(api);
    await loaded(c);
    await c.transition('confirmed');
    final s = await loaded(c);
    expect(s.error?.code, 'payment_required');
    expect(s.order.id, 1);
  });

  test('undo (packed → open) note shoho (8.8 v1.37)', () async {
    final api = FakeApi({
      'GET /api/admin/orders/1': [
        json(
          200,
          orderJson(
            1,
            status: 'confirmed',
            fulfillments: [fulfillmentJson(10, status: 'packed')],
          ),
        ),
      ],
      'POST /api/admin/fulfillments/10/transition': [
        json(200, {'status': 'open'}),
      ],
    });
    final c = cubit(api);
    await loaded(c);
    await c.transitionFulfillment(10, 'open', note: 'ojon bhul');
    expect(api.bodies['POST /api/admin/fulfillments/10/transition'], {
      'to': 'open',
      'note': 'ojon bhul',
    });
  });

  test('note jog (call)', () async {
    final api = FakeApi({
      'GET /api/admin/orders/1': [json(200, orderJson(1))],
      'POST /api/admin/orders/1/notes': [
        json(201, {
          'id': 5,
          'staff_id': 1,
          'kind': 'call',
          'note': 'phone dhoreni',
          'created_at': '2026-11-17T05:00:00Z',
        }),
      ],
    });
    final c = cubit(api);
    await loaded(c);
    await c.addNote('phone dhoreni', kind: 'call');
    expect(api.bodies['POST /api/admin/orders/1/notes'], {
      'kind': 'call',
      'note': 'phone dhoreni',
    });
  });

  test('print link: opener e POST er url (notun tab, 11.6)', () async {
    final api = FakeApi({
      'GET /api/admin/orders/1': [json(200, orderJson(1))],
      'POST /api/admin/print-links': [
        json(200, {
          'url': 'https://shop.test/print/invoice?token=t',
          'expires_at': '2026-11-17T05:10:00Z',
        }),
      ],
    });
    final c = cubit(api);
    await loaded(c);
    c.print('invoice');
    expect(opened, hasLength(1));
    expect(await opened.single, 'https://shop.test/print/invoice?token=t');
    expect(api.bodies['POST /api/admin/print-links'], {
      'kind': 'invoice',
      'order_ids': [1],
    });
  });

  group('page', () {
    Future<void> pump(
      WidgetTester tester,
      Map<String, Object?> order, {
      String role = 'owner',
      Size size = const Size(360, 740),
    }) async {
      final api = FakeApi({
        'GET /api/admin/orders/1': [json(200, order)],
      });
      await pumpPage(
        tester,
        BlocProvider(create: (_) => cubit(api), child: const OrderDetailPage()),
        role: role,
        size: size,
      );
    }

    testWidgets('360px e overflow nai, COD pending e confirm ar cancel', (
      tester,
    ) async {
      await pump(
        tester,
        orderJson(
          1,
          risk: ['cod_high_amount'],
          fulfillments: [fulfillmentJson(10)],
        ),
      );
      expect(tester.takeException(), isNull);
      expect(find.byKey(const Key('order.confirm')), findsOneWidget);
      expect(find.byKey(const Key('order.cancel')), findsOneWidget);
      expect(find.text('SB-1'), findsWidgets);
    });

    testWidgets('online unpaid e confirm button nai (payment_required)', (
      tester,
    ) async {
      await pump(tester, orderJson(1, method: 'sslcommerz'));
      expect(find.byKey(const Key('order.confirm')), findsNothing);
      expect(find.byKey(const Key('order.cancel')), findsOneWidget);
    });

    testWidgets('staff e cancel nai; packed e undo ar ship', (tester) async {
      await pump(
        tester,
        orderJson(
          1,
          status: 'confirmed',
          fulfillments: [fulfillmentJson(10, status: 'packed')],
        ),
        role: 'staff',
      );
      expect(find.byKey(const Key('order.cancel')), findsNothing);
      expect(find.byKey(const Key('fulfillment.10.undo')), findsOneWidget);
      expect(find.byKey(const Key('fulfillment.10.ship')), findsOneWidget);
    });

    testWidgets('undo e note dialog; khali note e POST na', (tester) async {
      final api = FakeApi({
        'GET /api/admin/orders/1': [
          json(
            200,
            orderJson(
              1,
              status: 'confirmed',
              fulfillments: [fulfillmentJson(10, status: 'packed')],
            ),
          ),
        ],
        'POST /api/admin/fulfillments/10/transition': [
          json(200, {'status': 'open'}),
        ],
      });
      await pumpPage(
        tester,
        BlocProvider(create: (_) => cubit(api), child: const OrderDetailPage()),
        size: const Size(360, 740),
      );
      await tester.ensureVisible(find.byKey(const Key('fulfillment.10.undo')));
      await tester.tap(find.byKey(const Key('fulfillment.10.undo')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('note.submit')));
      await tester.pumpAndSettle();
      expect(
        api.calls,
        isNot(contains('POST /api/admin/fulfillments/10/transition')),
      );
      await tester.enterText(find.byKey(const Key('note.field')), 'ojon bhul');
      await tester.tap(find.byKey(const Key('note.submit')));
      await tester.pumpAndSettle();
      expect(api.bodies['POST /api/admin/fulfillments/10/transition'], {
        'to': 'open',
        'note': 'ojon bhul',
      });
    });

    Future<FakeApi> pumpApi(
      WidgetTester tester,
      Map<String, List<dynamic>> routes, {
      String role = 'owner',
      String locale = 'bn',
    }) async {
      final api = FakeApi({
        for (final e in routes.entries) e.key: [...e.value.cast()],
      });
      await pumpPage(
        tester,
        BlocProvider(create: (_) => cubit(api), child: const OrderDetailPage()),
        role: role,
        size: const Size(360, 740),
        locale: locale,
      );
      return api;
    }

    final twoItems = orderJson(
      1,
      status: 'confirmed',
      fulfillments: [
        fulfillmentJson(10, items: [itemJson(1), itemJson(2)]),
      ],
    );

    testWidgets('order edit: shudhu kom, note lagbe, bodla item shudhu', (
      tester,
    ) async {
      final api = await pumpApi(tester, {
        'GET /api/admin/orders/1': [json(200, twoItems)],
        'PATCH /api/admin/orders/1/items': [
          json(200, {'status': 'confirmed', 'display_status': 'confirmed'}),
        ],
      });
      await tester.ensureVisible(find.byKey(const Key('order.edit')));
      await tester.tap(find.byKey(const Key('order.edit')));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      // barano na (qty 2)
      await tester.enterText(find.byKey(const Key('edit.1')), '3');
      await tester.enterText(
        find.byKey(const Key('edit.note')),
        'customer bollo',
      );
      await tester.tap(find.byKey(const Key('edit.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['PATCH /api/admin/orders/1/items'], isNull);
      // note chhara na
      await tester.enterText(find.byKey(const Key('edit.1')), '0');
      await tester.enterText(find.byKey(const Key('edit.note')), ' ');
      await tester.tap(find.byKey(const Key('edit.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['PATCH /api/admin/orders/1/items'], isNull);
      await tester.enterText(
        find.byKey(const Key('edit.note')),
        ' customer bollo ',
      );
      await tester.tap(find.byKey(const Key('edit.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['PATCH /api/admin/orders/1/items'], {
        'items': [
          {'item_id': 1, 'quantity': 0},
        ],
        'note': 'customer bollo',
      });
    });

    testWidgets('order edit: staff e 403 hole limit er lekha', (tester) async {
      await pumpApi(
        tester,
        {
          'GET /api/admin/orders/1': [json(200, twoItems)],
          'PATCH /api/admin/orders/1/items': [err(403, 'forbidden')],
        },
        role: 'staff',
        locale: 'en',
      );
      await tester.ensureVisible(find.byKey(const Key('order.edit')));
      await tester.tap(find.byKey(const Key('order.edit')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('edit.2')), '1');
      await tester.enterText(find.byKey(const Key('edit.note')), 'kom');
      await tester.tap(find.byKey(const Key('edit.save')));
      await tester.pumpAndSettle();
      expect(
        find.text(
          'Staff cannot remove all items of a seller or make a refund. Ask the owner or an admin.',
        ),
        findsOneWidget,
      );
    });

    final delivered = orderJson(
      1,
      status: 'confirmed',
      display: 'delivered',
      payment: 'paid',
      fulfillments: [
        fulfillmentJson(
          10,
          status: 'delivered',
          items: [itemJson(1), itemJson(2)],
        ),
      ],
    );

    testWidgets('return: restock item bechhe, note lagbe', (tester) async {
      final api = await pumpApi(tester, {
        'GET /api/admin/orders/1': [json(200, delivered)],
        'POST /api/admin/fulfillments/10/transition': [
          json(200, {'status': 'returned'}),
        ],
      });
      expect(find.byKey(const Key('order.edit')), findsNothing);
      await tester.ensureVisible(
        find.byKey(const Key('fulfillment.10.return')),
      );
      await tester.tap(find.byKey(const Key('fulfillment.10.return')));
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      await tester.tap(find.byKey(const Key('return.2')));
      await tester.tap(find.byKey(const Key('return.save')));
      await tester.pumpAndSettle();
      expect(
        api.calls,
        isNot(contains('POST /api/admin/fulfillments/10/transition')),
      );
      await tester.enterText(find.byKey(const Key('return.note')), 'nilo na');
      await tester.tap(find.byKey(const Key('return.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['POST /api/admin/fulfillments/10/transition'], {
        'to': 'returned',
        'note': 'nilo na',
        'restock_item_ids': [2],
      });
    });

    testWidgets('return: staff e refund 403 er lekha', (tester) async {
      await pumpApi(
        tester,
        {
          'GET /api/admin/orders/1': [json(200, delivered)],
          'POST /api/admin/fulfillments/10/transition': [err(403, 'forbidden')],
        },
        role: 'staff',
        locale: 'en',
      );
      await tester.ensureVisible(
        find.byKey(const Key('fulfillment.10.return')),
      );
      await tester.tap(find.byKey(const Key('fulfillment.10.return')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('return.note')), 'nilo na');
      await tester.tap(find.byKey(const Key('return.save')));
      await tester.pumpAndSettle();
      expect(
        find.text(
          'Money was taken for this parcel, so the return needs a refund. Only the owner or an admin can do it.',
        ),
        findsOneWidget,
      );
    });

    testWidgets('1280px e o overflow nai, en', (tester) async {
      await pump(
        tester,
        orderJson(1, fulfillments: [fulfillmentJson(10)]),
        size: const Size(1280, 900),
      );
      expect(tester.takeException(), isNull);
    });
  });
}
