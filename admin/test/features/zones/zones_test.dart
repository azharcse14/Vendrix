import 'package:admin/features/zones/bloc/zones_cubit.dart';
import 'package:admin/features/zones/pages/zones_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> zoneJson(
  int id, {
  String? area = 'Mirpur',
  bool active = true,
}) => {
  'id': id,
  'name': {'bn': 'Dhaka $id', 'en': 'Dhaka $id'},
  'district': 'Dhaka',
  'area': area,
  'charge': 6000,
  'min_order': 0,
  'free_delivery_over': null,
  'same_city': true,
  'is_active': active,
};

Future<FakeApi> pump(
  WidgetTester tester, {
  Map<String, List<dynamic>> extra = const {},
  Size size = const Size(1280, 900),
  String locale = 'bn',
}) async {
  final api = FakeApi({
    'GET /api/admin/delivery-zones': [
      json(200, page([zoneJson(1), zoneJson(2, area: null, active: false)])),
    ],
    for (final e in extra.entries) e.key: [...e.value.cast()],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => ZonesCubit(api.zones)..load(),
      child: const ZonesPage(),
    ),
    size: size,
    locale: locale,
  );
  return api;
}

void main() {
  for (final w in [360.0, 1280.0]) {
    testWidgets('list ${w.toInt()}px: inactive o, overflow nai', (
      tester,
    ) async {
      await pump(tester, size: Size(w, 800), locale: 'en');
      expect(tester.takeException(), isNull);
      expect(find.byKey(const Key('zone.1')), findsOneWidget);
      expect(find.byKey(const Key('zone.2')), findsOneWidget);
      expect(find.text('Inactive'), findsOneWidget);
      expect(
        find.textContaining('Whole district'),
        findsOneWidget,
      ); // area null
    });
  }

  testWidgets('create: name.bn lagbe, trim, taka poisha, area khali', (
    tester,
  ) async {
    final api = await pump(
      tester,
      extra: {
        'POST /api/admin/delivery-zones': [json(201, zoneJson(3))],
      },
    );
    await tester.tap(find.byKey(const Key('zone.new')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('zone.district')), ' Dhaka ');
    await tester.enterText(find.byKey(const Key('zone.charge')), '60');
    await tester.tap(find.byKey(const Key('zone.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/delivery-zones'], isNull); // naam nai

    await tester.enterText(
      find.byKey(const Key('zone.name_bn')),
      'Dhaka shohor',
    );
    await tester.enterText(find.byKey(const Key('zone.free_over')), '1,000');
    await tester.tap(find.byKey(const Key('zone.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/delivery-zones'], {
      'name': {'bn': 'Dhaka shohor', 'en': ''},
      'district': 'Dhaka',
      'area': null,
      'charge': 6000,
      'min_order': 0,
      'free_delivery_over': 100000,
      'same_city': true,
      'is_active': true,
    });
    expect(api.calls.last, 'GET /api/admin/delivery-zones'); // abar load
  });

  testWidgets('district 100 er beshi e submit na', (tester) async {
    final api = await pump(tester);
    await tester.tap(find.byKey(const Key('zone.new')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('zone.name_bn')), 'X');
    await tester.enterText(find.byKey(const Key('zone.district')), 'd' * 101);
    await tester.enterText(find.byKey(const Key('zone.charge')), '60');
    await tester.tap(find.byKey(const Key('zone.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/delivery-zones'], isNull);
  });

  testWidgets('edit: PATCH ager value shoho, inactive kora', (tester) async {
    final api = await pump(
      tester,
      extra: {
        'PATCH /api/admin/delivery-zones/1': [
          json(200, zoneJson(1, active: false)),
        ],
      },
    );
    await tester.tap(find.byKey(const Key('zone.1.edit')));
    await tester.pumpAndSettle();
    expect(find.text('Mirpur'), findsWidgets);
    await tester.tap(find.byKey(const Key('zone.active')));
    await tester.tap(find.byKey(const Key('zone.save')));
    await tester.pumpAndSettle();
    final body =
        api.bodies['PATCH /api/admin/delivery-zones/1']!
            as Map<String, Object?>;
    expect(body['is_active'], false);
    expect(body['area'], 'Mirpur');
    expect(body['charge'], 6000);
  });

  testWidgets('zone_exists er lekha', (tester) async {
    await pump(
      tester,
      locale: 'en',
      extra: {
        'POST /api/admin/delivery-zones': [err(409, 'zone_exists')],
      },
    );
    await tester.tap(find.byKey(const Key('zone.new')));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('zone.name_bn')), 'X');
    await tester.enterText(find.byKey(const Key('zone.district')), 'Dhaka');
    await tester.enterText(find.byKey(const Key('zone.charge')), '60');
    await tester.tap(find.byKey(const Key('zone.save')));
    await tester.pumpAndSettle();
    expect(find.text('This delivery area already exists'), findsOneWidget);
  });
}
