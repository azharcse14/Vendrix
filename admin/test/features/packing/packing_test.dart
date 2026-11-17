import 'package:admin/features/packing/bloc/pack_cubit.dart';
import 'package:admin/features/packing/bloc/pick_list_cubit.dart';
import 'package:admin/features/packing/pages/pack_page.dart';
import 'package:admin/features/packing/pages/packing_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> pickVariant(int id, {bool variable = false}) => {
  'variant_id': id,
  'product_name': {'bn': 'Chal', 'en': 'Rice'},
  'variant_label': {'bn': '1 kg', 'en': '1 kg'},
  'sku': 'SKU-$id',
  'variable_weight': variable,
  'quantity': 3,
  'ordered_weight_g': variable ? 3000 : null,
};

final pickList = {
  'date': '2026-11-17',
  'variants': [pickVariant(1), pickVariant(2, variable: true)],
  'areas': [
    {
      'district': 'Dhaka',
      'area': 'Mirpur',
      'variants': [pickVariant(1)],
    },
  ],
  'fulfillments': [
    {
      'id': 10,
      'order_id': 1,
      'order_number': 'SB-1',
      'seq': 1,
      'vendor_id': 1,
      'recipient_name': 'Karim Uddin',
      'district': 'Dhaka',
      'area': 'Mirpur',
      'confirmed_at': '2026-11-17T04:00:00Z',
    },
  ],
};

// Order 1, fulfillment 10: item 1 fixed, item 2 ojon-variable (1000 g).
final packOrder = orderJson(
  1,
  status: 'confirmed',
  fulfillments: [
    fulfillmentJson(10, items: [itemJson(1), itemJson(2, variable: true)]),
  ],
);

void main() {
  group('pick list (8.11 item 1)', () {
    test('load ar print (pick_list, date)', () async {
      final api = FakeApi({
        'GET /api/admin/packing/pick-list': [json(200, pickList)],
        'POST /api/admin/print-links': [
          json(200, {
            'url': 'https://shop.test/print/pick_list?token=t',
            'expires_at': '2026-11-17T05:10:00Z',
          }),
        ],
      });
      final opened = <Future<String>>[];
      final c = PickListCubit(api.orders, opened.add)..load();
      final s = await c.stream.firstWhere(
        (s) => s is PickListLoaded,
      ) as PickListLoaded;
      expect(s.list.fulfillments.single.orderNumber, 'SB-1');
      c.print();
      expect(await opened.single, 'https://shop.test/print/pick_list?token=t');
      expect(api.bodies['POST /api/admin/print-links'], {
        'kind': 'pick_list',
        'date': '2026-11-17',
      });
    });

    testWidgets('360px: overflow nai, fulfillment ar variant dekhay', (
      tester,
    ) async {
      final api = FakeApi({
        'GET /api/admin/packing/pick-list': [json(200, pickList)],
      });
      await pumpPage(
        tester,
        BlocProvider(
          create: (_) => PickListCubit(api.orders, (_) {})..load(),
          child: const PackingPage(),
        ),
        size: const Size(360, 740),
      );
      expect(tester.takeException(), isNull);
      expect(find.text('SB-1'), findsOneWidget);
      expect(find.byKey(const Key('pick.10.pack')), findsOneWidget);
      expect(find.byKey(const Key('pick.print')), findsOneWidget);
    });
  });

  group('pack (8.11 item 2, 3; 11.7 13 ii)', () {
    test('shudhu ojon-variable item er gram jay', () async {
      final api = FakeApi({
        'GET /api/admin/orders/1': [json(200, packOrder)],
        'POST /api/admin/fulfillments/10/pack': [
          json(200, {'status': 'packed', 'display_status': 'packed'}),
        ],
      });
      final c = PackCubit(api.orders, 1, 10)..load();
      final r = await c.stream.firstWhere((s) => s is PackReady) as PackReady;
      expect(r.weighed.map((i) => i.id), [2]);
      await c.submit({2: 1050});
      expect(c.state, isA<PackDone>());
      expect(api.bodies['POST /api/admin/fulfillments/10/pack'], {
        'items': [
          {'item_id': 2, 'packed_weight_g': 1050},
        ],
      });
    });

    test(
      '409 weight_over_limit → note chay, tarpor customer_ok_note shoho',
      () async {
        final api = FakeApi({
          'GET /api/admin/orders/1': [json(200, packOrder)],
          'POST /api/admin/fulfillments/10/pack': [
            err(409, 'weight_over_limit', null, {
              'items': [
                {'item_id': 2, 'ordered_weight_g': 1000, 'max_weight_g': 1100},
              ],
            }),
            json(200, {'status': 'packed', 'display_status': 'packed'}),
          ],
        });
        final c = PackCubit(api.orders, 1, 10)..load();
        await c.stream.firstWhere((s) => s is PackReady);
        await c.submit({2: 1200});
        final s = c.state as PackReady;
        expect(s.overLimit, isTrue);
        expect(s.error?.code, 'weight_over_limit');
        await c.submit({2: 1200}, customerOkNote: 'customer raji, phone e');
        expect(c.state, isA<PackDone>());
        expect(api.bodies['POST /api/admin/fulfillments/10/pack'], {
          'items': [
            {'item_id': 2, 'packed_weight_g': 1200},
          ],
          'customer_ok_note': 'customer raji, phone e',
        });
      },
    );

    testWidgets('page 360px: gram dile pack, 409 e note dialog', (
      tester,
    ) async {
      final api = FakeApi({
        'GET /api/admin/orders/1': [json(200, packOrder)],
        'POST /api/admin/fulfillments/10/pack': [
          err(409, 'weight_over_limit', null, {
            'items': [
              {'item_id': 2, 'ordered_weight_g': 1000, 'max_weight_g': 1100},
            ],
          }),
          json(200, {'status': 'packed', 'display_status': 'packed'}),
        ],
      });
      await pumpPage(
        tester,
        BlocProvider(
          create: (_) => PackCubit(api.orders, 1, 10)..load(),
          child: const PackPage(),
        ),
        size: const Size(360, 740),
      );
      expect(tester.takeException(), isNull);
      await tester.enterText(find.byKey(const Key('pack.item.2')), '1200');
      await tester.tap(find.byKey(const Key('pack.submit')));
      await tester.pumpAndSettle();
      expect(find.byKey(const Key('pack.note')), findsOneWidget);
      await tester.enterText(
        find.byKey(const Key('pack.note')),
        'customer raji',
      );
      await tester.tap(find.byKey(const Key('pack.note.submit')));
      await tester.pumpAndSettle();
      expect(
        (api.bodies['POST /api/admin/fulfillments/10/pack']!
            as Map)['customer_ok_note'],
        'customer raji',
      );
    });

    testWidgets('gram khali hole submit na (validation)', (tester) async {
      final api = FakeApi({
        'GET /api/admin/orders/1': [json(200, packOrder)],
      });
      await pumpPage(
        tester,
        BlocProvider(
          create: (_) => PackCubit(api.orders, 1, 10)..load(),
          child: const PackPage(),
        ),
        size: const Size(360, 740),
      );
      await tester.tap(find.byKey(const Key('pack.submit')));
      await tester.pumpAndSettle();
      expect(
        api.calls,
        isNot(contains('POST /api/admin/fulfillments/10/pack')),
      );
    });
  });
}
