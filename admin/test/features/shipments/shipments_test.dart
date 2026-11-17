import 'package:admin/features/shipments/bloc/shipments_cubit.dart';
import 'package:admin/features/shipments/pages/shipments_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> row(
  int id,
  String status, {
  Map<String, Object?>? shipment,
}) => {
  'id': id,
  'order_id': id,
  'order_number': 'SB-$id',
  'vendor_id': 1,
  'seq': 1,
  'status': status,
  'total': 125000,
  'recipient_name': 'Karim Uddin',
  'district': 'Dhaka',
  'area': 'Mirpur',
  'placed_at': '2026-11-17T04:00:00Z',
  'shipment': shipment,
};

const shipment = {
  'id': 7,
  'carrier': 'steadfast',
  'tracking_code': 'TR1',
  'status': 'booked',
  'cod_amount': 125000,
};

void main() {
  FakeApi api0() => FakeApi({
    'GET /api/admin/fulfillments': [
      json(200, page([row(1, 'packed')])),
      json(
        200,
        page([row(2, 'shipped', shipment: shipment), row(3, 'shipped')]),
      ),
    ],
  });

  test('load: packed (pathano baki) ar shipped (shipment shoho)', () async {
    final api = api0();
    final c = ShipmentsCubit(api.orders)..load();
    final s = await c.stream.firstWhere(
      (s) => s is ShipmentsLoaded,
    ) as ShipmentsLoaded;
    expect(s.toShip.map((r) => r.id), [
      1,
      3,
    ]); // 3: shipped, active shipment nai (abar pathano)
    expect(s.onTheWay.single.shipment?.id, 7);
    expect(api.calls, [
      'GET /api/admin/fulfillments?status=packed&limit=100',
      'GET /api/admin/fulfillments?status=shipped&limit=100',
    ]);
  });

  test('ship: carrier ar tracking (trim), tarpor abar load', () async {
    final api = api0()
      ..routes['POST /api/admin/fulfillments/1/shipments'] = [
        json(201, {
          ...shipment,
          'fulfillment_id': 1,
          'display_status': 'shipped',
        }),
      ];
    final c = ShipmentsCubit(api.orders)..load();
    await c.stream.firstWhere((s) => s is ShipmentsLoaded);
    await c.ship(1, 'steadfast', tracking: ' TR1 ');
    expect(api.bodies['POST /api/admin/fulfillments/1/shipments'], {
      'carrier': 'steadfast',
      'tracking_code': 'TR1',
    });
    await c.ship(1, 'own_rider', tracking: '');
    expect(api.bodies['POST /api/admin/fulfillments/1/shipments'], {
      'carrier': 'own_rider',
    });
  });

  test('delivered, returned, cancelled: PATCH shipment', () async {
    final api = api0()
      ..routes['PATCH /api/admin/shipments/7'] = [
        json(200, {
          'id': 7,
          'status': 'delivered',
          'display_status': 'delivered',
        }),
      ];
    final c = ShipmentsCubit(api.orders)..load();
    await c.stream.firstWhere((s) => s is ShipmentsLoaded);
    await c.updateShipment(7, 'delivered');
    expect(api.bodies['PATCH /api/admin/shipments/7'], {'status': 'delivered'});
  });

  test('bhul (tracking taken) state e', () async {
    final api = api0()
      ..routes['POST /api/admin/fulfillments/1/shipments'] = [
        err(400, 'validation_failed', {'tracking_code': 'taken'}),
      ];
    final c = ShipmentsCubit(api.orders)..load();
    await c.stream.firstWhere((s) => s is ShipmentsLoaded);
    await c.ship(1, 'steadfast', tracking: 'TR1');
    expect((c.state as ShipmentsLoaded).error?.fields, {
      'tracking_code': 'taken',
    });
  });

  for (final (role, deliver) in [('staff', true), ('vendor_staff', false)]) {
    testWidgets(
      'page 360px $role: overflow nai, delivered ${deliver ? 'ache' : 'nai'}',
      (tester) async {
        final api = api0();
        await pumpPage(
          tester,
          BlocProvider(
            create: (_) => ShipmentsCubit(api.orders)..load(),
            child: const ShipmentsPage(),
          ),
          role: role,
          size: const Size(360, 740),
        );
        expect(tester.takeException(), isNull);
        expect(find.byKey(const Key('ship.1')), findsOneWidget);
        expect(find.text('TR1'), findsOneWidget);
        // vendor role e shipped → delivered na (CanTransitionFulfillment)
        expect(
          find.byKey(const Key('shipment.7.delivered')),
          deliver ? findsOneWidget : findsNothing,
        );
      },
    );
  }

  testWidgets('ship form: carrier, tracking → POST', (tester) async {
    final api = api0()
      ..routes['POST /api/admin/fulfillments/1/shipments'] = [
        json(201, {
          ...shipment,
          'fulfillment_id': 1,
          'display_status': 'shipped',
        }),
      ];
    await pumpPage(
      tester,
      BlocProvider(
        create: (_) => ShipmentsCubit(api.orders)..load(),
        child: const ShipmentsPage(),
      ),
      size: const Size(360, 740),
    );
    await tester.tap(find.byKey(const Key('ship.1')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('ship.tracking')), 'TR9');
    await tester.tap(find.byKey(const Key('ship.submit')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/fulfillments/1/shipments'], {
      'carrier': 'own_rider',
      'tracking_code': 'TR9',
    });
  });
}
