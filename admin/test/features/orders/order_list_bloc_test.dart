import 'package:admin/features/orders/bloc/order_list_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

void main() {
  Future<OrderListState> next(OrderListBloc b) =>
      b.stream.firstWhere((s) => s is! OrderListLoading);

  test('shuru: shob order, page 1 (11.4)', () async {
    final api = FakeApi({
      'GET /api/admin/orders': [
        json(200, page([orderJson(1), orderJson(2)], total: 2)),
      ],
    });
    final b = OrderListBloc(api.orders)..add(OrderListStarted());
    final s = await next(b) as OrderListLoaded;
    expect(s.page.items.map((o) => o.id), [1, 2]);
    expect(s.page.hasNext, isFalse);
    expect(api.calls, ['GET /api/admin/orders?page=1']);
  });

  test('tab: status = display_status, "confirm er opekkhay" ar "deadline kache" tab param (11.7 13 vii)', () async {
    final api = FakeApi({
      'GET /api/admin/orders': [json(200, page([]))],
    });
    final b = OrderListBloc(api.orders);
    for (final t in ['packed', 'awaiting_confirm', 'deadline_soon', 'all']) {
      b.add(OrderTabSelected(t));
      final s = await next(b);
      expect(s.filter.tab, t);
    }
    expect(api.calls, [
      'GET /api/admin/orders?status=packed&page=1',
      'GET /api/admin/orders?tab=awaiting_confirm&page=1',
      'GET /api/admin/orders?tab=deadline_soon&page=1',
      'GET /api/admin/orders?page=1',
    ]);
    expect(
      orderTabs,
      containsAll([
        'all',
        'awaiting_confirm',
        'deadline_soon',
        'pending',
        'cancelled',
      ]),
    );
  });

  test('q (number ba phone, trim) ar risk filter tab er sathe thake', () async {
    final api = FakeApi({
      'GET /api/admin/orders': [json(200, page([]))],
    });
    final b = OrderListBloc(api.orders)..add(OrderTabSelected('pending'));
    await next(b);
    b.add(OrderSearchSubmitted(' 01711000001 '));
    await next(b);
    b.add(OrderRiskToggled());
    final s = await next(b);
    expect(s.filter.risk, isTrue);
    expect(s.filter.q, '01711000001');
    expect(
      api.calls.last,
      'GET /api/admin/orders?status=pending&risk=1&q=01711000001&page=1',
    );
    b.add(OrderSearchSubmitted(''));
    await next(b);
    expect(
      api.calls.last,
      'GET /api/admin/orders?status=pending&risk=1&page=1',
    );
  });

  test('paging: porer/ager page (OFFSET, tai "aro" na; advisor)', () async {
    final api = FakeApi({
      'GET /api/admin/orders': [
        json(200, page([orderJson(1)], total: 21)),
        json(200, page([orderJson(2)], page: 2, total: 21)),
      ],
    });
    final b = OrderListBloc(api.orders)..add(OrderListStarted());
    final p1 = await next(b) as OrderListLoaded;
    expect((p1.page.hasNext, p1.page.page), (true, 1));
    b.add(OrderPageRequested(2));
    final p2 = await next(b) as OrderListLoaded;
    expect(p2.page.items.map((o) => o.id), [2]);
    expect(p2.page.hasNext, isFalse);
    expect(api.calls.last, 'GET /api/admin/orders?page=2');
  });

  test('API bhul → OrderListFailed (filter thake)', () async {
    final api = FakeApi({
      'GET /api/admin/orders': [err(500, 'internal')],
    });
    final b = OrderListBloc(api.orders)..add(OrderTabSelected('shipped'));
    final s = await next(b) as OrderListFailed;
    expect(s.error.code, 'internal');
    expect(s.filter.tab, 'shipped');
  });
}
