import 'package:admin/features/audit/bloc/audit_cubit.dart';
import 'package:admin/features/audit/pages/audit_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> auditJson(int id) => {
  'id': id,
  'staff_id': 1,
  'staff_name': 'Rahim',
  'action': 'staff.update',
  'entity_type': 'staff',
  'entity_id': 2,
  'before': {'role': 'staff'},
  'after': {'role': 'admin'},
  'ip': '127.0.0.1',
  'created_at': '2026-11-17T04:00:00Z',
};

Future<FakeApi> pump(
  WidgetTester tester, {
  Size size = const Size(1280, 900),
}) async {
  final api = FakeApi({
    'GET /api/admin/audit-logs': [
      json(200, page([auditJson(1)])),
    ],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => AuditCubit(api.audit)..load(),
      child: const AuditPage(),
    ),
    size: size,
  );
  return api;
}

void main() {
  for (final w in [360.0, 1280.0]) {
    testWidgets('list ${w.toInt()}px: overflow nai', (tester) async {
      await pump(tester, size: Size(w, 800));
      expect(tester.takeException(), isNull);
      expect(find.text('staff.update'), findsOneWidget);
    });
  }

  testWidgets('filter: action, staff_id, from, to', (tester) async {
    final api = await pump(tester);
    await tester.enterText(
      find.byKey(const Key('audit.action')),
      ' staff.update ',
    );
    await tester.enterText(find.byKey(const Key('audit.staff')), '1');
    await tester.enterText(find.byKey(const Key('audit.from')), '2026-11-01');
    await tester.enterText(find.byKey(const Key('audit.to')), '2026-11-17');
    await tester.tap(find.byKey(const Key('audit.apply')));
    await tester.pumpAndSettle();
    expect(
      api.calls.last,
      'GET /api/admin/audit-logs?action=staff.update&staff_id=1'
      '&from=2026-11-01&to=2026-11-17',
    );
  });

  testWidgets('bhul tarikh e GET na', (tester) async {
    final api = await pump(tester);
    await tester.enterText(find.byKey(const Key('audit.from')), '1/11/2026');
    await tester.tap(find.byKey(const Key('audit.apply')));
    await tester.pumpAndSettle();
    expect(api.calls, ['GET /api/admin/audit-logs']);
  });

  testWidgets('row khulle before/after JSON', (tester) async {
    await pump(tester);
    await tester.tap(find.text('staff.update'));
    await tester.pumpAndSettle();
    expect(find.textContaining('"role": "staff"'), findsOneWidget);
    expect(find.textContaining('"role": "admin"'), findsOneWidget);
  });
}
