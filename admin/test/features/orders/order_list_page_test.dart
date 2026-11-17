import 'package:admin/features/orders/bloc/order_list_bloc.dart';
import 'package:admin/features/orders/pages/order_list_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

void main() {
  for (final (w, locale) in [(360.0, 'bn'), (360.0, 'en'), (1280.0, 'bn')]) {
    testWidgets(
      'order list ${w.toInt()}px $locale: overflow nai, risk chip, phone link',
      (tester) async {
        final api = FakeApi({
          'GET /api/admin/orders': [
            json(
              200,
              page([
                orderJson(1, risk: ['cod_high_amount', 'many_orders']),
                orderJson(2, status: 'confirmed'),
              ], total: 30),
            ),
          ],
        });
        await pumpPage(
          tester,
          BlocProvider(
            create: (_) => OrderListBloc(api.orders)..add(OrderListStarted()),
            child: const OrderListPage(),
          ),
          size: Size(w, 800),
          locale: locale,
        );
        expect(tester.takeException(), isNull);
        expect(find.text('SB-1'), findsOneWidget);
        expect(
          find.byKey(const Key('order.1.risk.cod_high_amount')),
          findsOneWidget,
        );
        expect(find.byKey(const Key('order.1.call')), findsOneWidget);
        expect(find.byKey(const Key('orders.next')), findsOneWidget);
        await tester.tap(find.byKey(const Key('order.1.call')));
        expect(openedUrls, ['tel:+8801711000001']);
      },
    );
  }

  testWidgets('tab bodlale bloc e OrderTabSelected', (tester) async {
    final api = FakeApi({
      'GET /api/admin/orders': [json(200, page([]))],
    });
    await pumpPage(
      tester,
      BlocProvider(
        create: (_) => OrderListBloc(api.orders)..add(OrderListStarted()),
        child: const OrderListPage(),
      ),
      size: const Size(360, 800),
    );
    await tester.tap(find.byKey(const Key('orders.tab.awaiting_confirm')));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'GET /api/admin/orders?tab=awaiting_confirm&page=1');
  });
}
