import 'package:admin/features/dashboard/bloc/dashboard_cubit.dart';
import 'package:admin/features/dashboard/pages/dashboard_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared/shared.dart';

import '../../helpers.dart';

const dash = {
  'date': '2026-11-17',
  'orders': 12,
  'sales': 1250000,
  'pending': 3,
  'to_pack': 5,
  'low_stock': 2,
  'expiring': 1,
  'deadline_soon': 4,
};

void main() {
  test('dashboard count load (11.7 13 viii)', () async {
    final api = FakeApi({
      'GET /api/admin/dashboard': [json(200, dash)],
    });
    final c = DashboardCubit(DashboardRepository(api.client))..load();
    final s = await c.stream.firstWhere(
      (s) => s is DashboardLoaded,
    ) as DashboardLoaded;
    expect((s.data.orders, s.data.pending, s.data.toPack), (12, 3, 5));
  });

  for (final (w, locale) in [(360.0, 'bn'), (1280.0, 'en')]) {
    testWidgets(
      'page ${w.toInt()}px $locale: count, bikri taka, overflow nai',
      (tester) async {
        final api = FakeApi({
          'GET /api/admin/dashboard': [json(200, dash)],
        });
        await pumpPage(
          tester,
          BlocProvider(
            create: (_) =>
                DashboardCubit(DashboardRepository(api.client))..load(),
            child: const DashboardPage(),
          ),
          size: Size(w, 800),
          locale: locale,
        );
        expect(tester.takeException(), isNull);
        expect(
          find.text(locale == 'bn' ? '৳১২,৫০০' : '৳12,500'),
          findsOneWidget,
        ); // rule4:ok taka er udahoron
        expect(find.byKey(const Key('dashboard.pending')), findsOneWidget);
        expect(find.byKey(const Key('dashboard.to_pack')), findsOneWidget);
      },
    );
  }
}
