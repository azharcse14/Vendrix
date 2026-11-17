import 'package:admin/features/coupons/bloc/coupons_cubit.dart';
import 'package:admin/features/coupons/pages/coupons_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> couponJson(
  int id, {
  String code = 'EID10',
  int used = 0,
  int? limit,
  bool active = true,
}) => {
  'id': id,
  'code': code,
  'kind': 'percent',
  'value': 10,
  'min_order': 50000,
  'max_discount': null,
  'usage_limit': limit,
  'used_count': used,
  'per_phone_limit': 1,
  'starts_at': null,
  'expires_at': '2026-12-31T17:59:59Z',
  'is_active': active,
  'created_at': '2026-11-17T04:00:00Z',
};

Future<FakeApi> pump(
  WidgetTester tester, {
  Map<String, List<dynamic>> extra = const {},
  Size size = const Size(1280, 900),
  String locale = 'bn',
}) async {
  final api = FakeApi({
    'GET /api/admin/coupons': [
      json(
        200,
        page([couponJson(1), couponJson(2, code: 'USED5', used: 3, limit: 10)]),
      ),
    ],
    for (final e in extra.entries) e.key: [...e.value.cast()],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => CouponsCubit(api.coupons)..load(),
      child: const CouponsPage(),
    ),
    size: size,
    locale: locale,
  );
  return api;
}

Future<void> fillNew(WidgetTester tester) async {
  await tester.tap(find.byKey(const Key('coupon.new')));
  await tester.pumpAndSettle();
  await tester.enterText(find.byKey(const Key('coupon.code')), ' winter-26 ');
  await tester.enterText(find.byKey(const Key('coupon.value')), '15');
}

void main() {
  for (final w in [360.0, 1280.0]) {
    testWidgets('list ${w.toInt()}px: overflow nai, bebohar', (tester) async {
      await pump(tester, size: Size(w, 800));
      expect(tester.takeException(), isNull);
      expect(find.text('EID10'), findsOneWidget);
      expect(find.text('USED5'), findsOneWidget);
    });
  }

  testWidgets('create: code upper + trim, percent, tarikh shop er din', (
    tester,
  ) async {
    final api = await pump(
      tester,
      extra: {
        'POST /api/admin/coupons': [
          json(201, couponJson(3, code: 'WINTER-26')),
        ],
      },
    );
    await fillNew(tester);
    await tester.enterText(
      find.byKey(const Key('coupon.expires')),
      '2026-12-31',
    );
    await tester.tap(find.byKey(const Key('coupon.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/coupons'], {
      'code': 'WINTER-26',
      'kind': 'percent',
      'value': 15,
      'min_order': 0,
      'max_discount': null,
      'usage_limit': null,
      'per_phone_limit': 1,
      'starts_at': null,
      'expires_at': '2026-12-31T23:59:59+06:00',
      'is_active': true,
    });
  });

  testWidgets('bhul code ar percent > 100 e submit na', (tester) async {
    final api = await pump(tester);
    await fillNew(tester);
    await tester.enterText(find.byKey(const Key('coupon.code')), 'eid 10');
    await tester.tap(find.byKey(const Key('coupon.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/coupons'], isNull);
    await tester.enterText(find.byKey(const Key('coupon.code')), 'EID');
    await tester.enterText(find.byKey(const Key('coupon.value')), '101');
    await tester.tap(find.byKey(const Key('coupon.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/coupons'], isNull);
  });

  testWidgets('fixed: value taka theke poisha', (tester) async {
    final api = await pump(
      tester,
      extra: {
        'POST /api/admin/coupons': [json(201, couponJson(3))],
      },
    );
    await fillNew(tester);
    await tester.tap(find.byKey(const Key('coupon.kind')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('kind.fixed')).last);
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const Key('coupon.value')), '50.50');
    await tester.tap(find.byKey(const Key('coupon.save')));
    await tester.pumpAndSettle();
    final body = api.bodies['POST /api/admin/coupons']! as Map<String, Object?>;
    expect(body['kind'], 'fixed');
    expect(body['value'], 5050);
  });

  testWidgets('bebohar hoyeche: code/kind/value lock, limit used er niche na', (
    tester,
  ) async {
    final api = await pump(
      tester,
      extra: {
        'PATCH /api/admin/coupons/2': [json(200, couponJson(2, used: 3))],
      },
    );
    await tester.tap(find.byKey(const Key('coupon.2.edit')));
    await tester.pumpAndSettle();
    for (final k in ['coupon.code', 'coupon.value']) {
      expect(
        tester
            .widget<TextField>(
              find.descendant(
                of: find.byKey(Key(k)),
                matching: find.byType(TextField),
              ),
            )
            .enabled,
        isFalse,
        reason: k,
      );
    }
    await tester.enterText(find.byKey(const Key('coupon.usage_limit')), '2');
    await tester.tap(find.byKey(const Key('coupon.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['PATCH /api/admin/coupons/2'], isNull); // 2 < 3
    await tester.enterText(find.byKey(const Key('coupon.usage_limit')), '');
    await tester.tap(find.byKey(const Key('coupon.save')));
    await tester.pumpAndSettle();
    final body =
        api.bodies['PATCH /api/admin/coupons/2']! as Map<String, Object?>;
    expect(body['code'], 'USED5'); // ekoi value, server lock e chole
    expect(body['usage_limit'], isNull);
  });

  testWidgets('server er below_used ar locked er lekha', (tester) async {
    await pump(
      tester,
      locale: 'en',
      extra: {
        'PATCH /api/admin/coupons/1': [
          err(400, 'validation_failed', {'usage_limit': 'below_used'}),
          err(400, 'validation_failed', {'value': 'locked'}),
        ],
      },
    );
    for (final want in [
      'Usage limit cannot be below the times already used',
      'Code, type and value cannot change after the coupon is used',
    ]) {
      await tester.tap(find.byKey(const Key('coupon.1.edit')));
      await tester.pumpAndSettle();
      await tester.tap(find.byKey(const Key('coupon.save')));
      await tester.pumpAndSettle();
      expect(find.text(want), findsOneWidget);
    }
  });

  testWidgets('coupon_exists er lekha', (tester) async {
    await pump(
      tester,
      locale: 'en',
      extra: {
        'POST /api/admin/coupons': [err(409, 'coupon_exists')],
      },
    );
    await fillNew(tester);
    await tester.tap(find.byKey(const Key('coupon.save')));
    await tester.pumpAndSettle();
    expect(find.text('A coupon with this code already exists'), findsOneWidget);
  });

  testWidgets('active switch: PATCH shudhu is_active', (tester) async {
    final api = await pump(
      tester,
      extra: {
        'PATCH /api/admin/coupons/1': [json(200, couponJson(1, active: false))],
      },
    );
    await tester.tap(find.byKey(const Key('coupon.1.active')));
    await tester.pumpAndSettle();
    expect(api.bodies['PATCH /api/admin/coupons/1'], {'is_active': false});
  });
}
