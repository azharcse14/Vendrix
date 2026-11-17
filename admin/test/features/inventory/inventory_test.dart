import 'package:admin/features/inventory/bloc/inventory_list_cubit.dart';
import 'package:admin/features/inventory/bloc/variant_stock_cubit.dart';
import 'package:admin/features/inventory/pages/inventory_page.dart';
import 'package:admin/features/inventory/pages/variant_stock_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> variantRow(int id, {int stock = 10}) => {
  'variant_id': id,
  'product_id': 2,
  'product_name': {'bn': 'Chal', 'en': 'Rice'},
  'variant_label': {
    'weight': {'bn': '1 kg', 'en': '1 kg'},
  },
  'sku': 'SKU-$id',
  'stock': stock,
  'low_stock_at': 5,
  'batch_remaining': stock,
  'next_expiry': '2026-12-01',
};

Map<String, Object?> batchJson(int id, {int? cost}) => {
  'id': id,
  'variant_id': 3,
  'batch_code': 'B$id',
  'quantity_received': 10,
  'quantity_remaining': 4,
  'cost_price': ?cost,
  'harvest_date': null,
  'expiry_date': '2026-12-01',
  'received_at': '2026-11-10T04:00:00Z',
};

Map<String, Object?> movementJson(int id, {int? cost}) => {
  'id': id,
  'variant_id': 3,
  'batch_id': 1,
  'change': -2,
  'reason': 'wastage',
  'order_id': null,
  'staff_id': 1,
  'note': 'pocha',
  'cost_price': ?cost,
  'created_at': '2026-11-11T04:00:00Z',
};

Future<FakeApi> pumpStock(
  WidgetTester tester, {
  String role = 'owner',
  Map<String, List<dynamic>> extra = const {},
  String locale = 'bn',
  Size size = const Size(1280, 900),
}) async {
  final staff = role == 'staff';
  final api = FakeApi({
    'GET /api/admin/variants/3/batches': [
      json(200, page([batchJson(1, cost: staff ? null : 9000)])),
    ],
    'GET /api/admin/inventory/movements': [
      json(200, page([movementJson(1, cost: staff ? null : 9000)])),
    ],
    for (final e in extra.entries) e.key: [...e.value.cast()],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => VariantStockCubit(api.inventory, 3)..load(),
      child: const VariantStockPage(),
    ),
    role: role,
    locale: locale,
    size: size,
  );
  return api;
}

void main() {
  for (final w in [360.0, 1280.0]) {
    testWidgets(
      'list ${w.toInt()}px: overflow nai, low chinho, khoja, low filter',
      (tester) async {
        final api = FakeApi({
          'GET /api/admin/inventory/variants': [
            json(200, page([variantRow(3), variantRow(4, stock: 2)])),
          ],
        });
        await pumpPage(
          tester,
          BlocProvider(
            create: (_) => InventoryListCubit(api.inventory)..load(),
            child: const InventoryPage(),
          ),
          size: Size(w, 800),
        );
        expect(tester.takeException(), isNull);
        expect(find.text('SKU-3'), findsOneWidget);
        expect(find.byKey(const Key('variant.4.low')), findsOneWidget);
        expect(find.byKey(const Key('variant.3.low')), findsNothing);
        await tester.enterText(
          find.byKey(const Key('inventory.search')),
          ' chal ',
        );
        await tester.testTextInput.receiveAction(TextInputAction.search);
        await tester.pumpAndSettle();
        expect(api.calls.last, 'GET /api/admin/inventory/variants?q=chal');
        await tester.tap(find.byKey(const Key('inventory.low')));
        await tester.pumpAndSettle();
        expect(
          api.calls.last,
          'GET /api/admin/inventory/variants?q=chal&low=1',
        );
      },
    );
  }

  testWidgets('variant: batch ar movement load, owner e kena dam', (
    tester,
  ) async {
    final api = await pumpStock(tester, size: const Size(360, 800));
    expect(tester.takeException(), isNull);
    expect(api.calls, [
      'GET /api/admin/variants/3/batches',
      'GET /api/admin/inventory/movements?variant_id=3',
    ]);
    expect(find.text('B1'), findsOneWidget);
    expect(find.byKey(const Key('batch.1.cost')), findsOneWidget);
  });

  testWidgets('staff: kena dam nai, batch dialog e cost_price field nai', (
    tester,
  ) async {
    await pumpStock(tester, role: 'staff');
    expect(find.byKey(const Key('batch.1.cost')), findsNothing);
    await tester.tap(find.byKey(const Key('stock.add')));
    await tester.pumpAndSettle();
    expect(find.byKey(const Key('batch.quantity')), findsOneWidget);
    expect(find.byKey(const Key('batch.cost')), findsNothing);
  });

  testWidgets('stock entry (batch): bhul din/qty e POST na; body', (
    tester,
  ) async {
    final api = await pumpStock(
      tester,
      extra: {
        'POST /api/admin/batches': [json(201, batchJson(2, cost: 8050))],
      },
    );
    await tester.tap(find.byKey(const Key('stock.add')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('batch.quantity')), '0');
    await tester.enterText(find.byKey(const Key('batch.expiry')), '2026-13-40');
    await tester.tap(find.byKey(const Key('batch.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/batches'], isNull);

    await tester.enterText(find.byKey(const Key('batch.quantity')), '12');
    await tester.enterText(find.byKey(const Key('batch.cost')), '80.50');
    await tester.enterText(find.byKey(const Key('batch.code')), ' B2 ');
    await tester.enterText(find.byKey(const Key('batch.expiry')), '2026-12-31');
    await tester.tap(find.byKey(const Key('batch.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/batches'], {
      'variant_id': 3,
      'quantity': 12,
      'cost_price': 8050,
      'batch_code': 'B2',
      'expiry_date': '2026-12-31',
    });
    expect(api.calls.last, 'GET /api/admin/inventory/movements?variant_id=3');
  });

  testWidgets(
    'komano: note chhara POST na; change negative; out_of_stock e available',
    (tester) async {
      final api = await pumpStock(
        tester,
        role: 'staff',
        locale: 'en',
        extra: {
          'POST /api/admin/inventory/adjust': [
            err(409, 'out_of_stock', null, {'available': 2}),
            json(200, {'stock': 0}),
          ],
        },
      );
      await tester.tap(find.byKey(const Key('stock.reduce')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('adjust.quantity')), '3');
      await tester.tap(find.byKey(const Key('adjust.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['POST /api/admin/inventory/adjust'], isNull);

      await tester.enterText(find.byKey(const Key('adjust.note')), ' pocha ');
      await tester.tap(find.byKey(const Key('adjust.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['POST /api/admin/inventory/adjust'], {
        'variant_id': 3,
        'change': -3,
        'reason': 'wastage',
        'note': 'pocha',
      });
      expect(find.text('Only 2 in stock'), findsOneWidget);
    },
  );
}
