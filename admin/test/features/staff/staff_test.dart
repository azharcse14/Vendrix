import 'package:admin/features/staff/bloc/staff_cubit.dart';
import 'package:admin/features/staff/pages/staff_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> staffJson(
  int id, {
  String email = 's@example.com',
  String role = 'staff',
  bool active = true,
  String? temp,
}) => {
  'id': id,
  'email': email,
  'name': 'Staff $id',
  'role': role,
  'vendor_id': null,
  'is_active': active,
  'must_change_password': temp != null,
  'totp_enabled': temp == null,
  'locked_until': null,
  'last_login_at': null,
  'created_at': '2026-11-17T04:00:00Z',
  'temp_password': ?temp,
};

Future<FakeApi> pump(
  WidgetTester tester, {
  Map<String, List<dynamic>> extra = const {},
  Size size = const Size(1280, 900),
  String locale = 'bn',
}) async {
  final api = FakeApi({
    'GET /api/admin/staff': [
      json(
        200,
        page([
          staffJson(1, email: 'n@example.com', role: 'owner'), // nije (profile)
          staffJson(2),
        ]),
      ),
    ],
    for (final e in extra.entries) e.key: [...e.value],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => StaffCubit(api.staff)..load(),
      child: const StaffPage(),
    ),
    size: size,
    locale: locale,
  );
  return api;
}

void main() {
  for (final w in [360.0, 1280.0]) {
    testWidgets('list ${w.toInt()}px: overflow nai, nijer row e kaj nai', (
      tester,
    ) async {
      await pump(tester, size: Size(w, 800));
      expect(tester.takeException(), isNull);
      expect(find.text('Staff 2'), findsOneWidget);
      expect(find.byKey(const Key('staff.2.reset')), findsOneWidget);
      expect(find.byKey(const Key('staff.1.reset')), findsNothing);
      expect(find.byKey(const Key('staff.1.active')), findsNothing);
    });
  }

  testWidgets('create: temp password ekbar dekhay, bondho korle ar nai', (
    tester,
  ) async {
    final api = await pump(
      tester,
      extra: {
        'POST /api/admin/staff': [
          json(
            201,
            staffJson(3, email: 'k@example.com', temp: 'TEMPPASSWORD26'),
          ),
        ],
      },
    );
    await tester.tap(find.byKey(const Key('staff.new')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('staff.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/staff'], isNull); // khali form

    await tester.enterText(
      find.byKey(const Key('staff.email')),
      ' k@example.com ',
    );
    await tester.enterText(find.byKey(const Key('staff.name')), 'Karim');
    await tester.tap(find.byKey(const Key('staff.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/staff'], {
      'email': 'k@example.com',
      'name': 'Karim',
      'role': 'staff',
    });
    expect(find.text('TEMPPASSWORD26'), findsOneWidget);
    await tester.tap(find.byKey(const Key('temp.close')));
    await tester.pumpAndSettle();
    expect(find.text('TEMPPASSWORD26'), findsNothing);
    expect(api.calls.last, 'GET /api/admin/staff'); // abar load
  });

  testWidgets('create: staff_exists er lekha', (tester) async {
    await pump(
      tester,
      locale: 'en',
      extra: {
        'POST /api/admin/staff': [err(409, 'staff_exists')],
      },
    );
    await tester.tap(find.byKey(const Key('staff.new')));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const Key('staff.email')),
      's@example.com',
    );
    await tester.enterText(find.byKey(const Key('staff.name')), 'Karim');
    await tester.tap(find.byKey(const Key('staff.save')));
    await tester.pumpAndSettle();
    expect(
      find.text('A staff member with this email already exists'),
      findsOneWidget,
    );
  });

  testWidgets('reset: confirm er por notun temp password', (tester) async {
    final api = await pump(
      tester,
      extra: {
        'POST /api/admin/staff/2/reset': [
          json(200, {'temp_password': 'NEWTEMP'}),
        ],
      },
    );
    await tester.tap(find.byKey(const Key('staff.2.reset')));
    await tester.pumpAndSettle();
    expect(api.calls, ['GET /api/admin/staff']);
    await tester.tap(find.byKey(const Key('confirm.ok')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('POST /api/admin/staff/2/reset'));
    expect(find.text('NEWTEMP'), findsOneWidget);
  });

  testWidgets('role bodol ar active bondho: PATCH shudhu oi field', (
    tester,
  ) async {
    final api = await pump(
      tester,
      extra: {
        'PATCH /api/admin/staff/2': [
          json(200, staffJson(2, role: 'admin')),
          json(200, staffJson(2, role: 'admin', active: false)),
        ],
      },
    );
    await tester.tap(find.byKey(const Key('staff.2.role')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('role.admin')).hitTestable());
    await tester.pumpAndSettle();
    expect(api.bodies['PATCH /api/admin/staff/2'], {'role': 'admin'});
    await tester.tap(find.byKey(const Key('staff.2.active')));
    await tester.pumpAndSettle();
    expect(api.bodies['PATCH /api/admin/staff/2'], {'is_active': false});
  });
}
