import 'package:admin/core/auth/auth_bloc.dart';
import 'package:admin/core/auth/session_client.dart';
import 'package:admin/core/widgets/app_shell.dart';
import 'package:admin/router.dart';
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:go_router/go_router.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';

import '../helpers.dart';

void main() {
  test('SessionClient: 401 e callback, baki response hubohu', () async {
    var n = 0;
    final c = SessionClient(
      MockClient(
        (req) async => http.Response('', req.url.path == '/x' ? 401 : 200),
      ),
      () => n++,
    );
    expect((await c.get(Uri.parse('https://a.test/x'))).statusCode, 401);
    expect((await c.get(Uri.parse('https://a.test/y'))).statusCode, 200);
    expect(n, 1);
  });

  test(
    'screen er 401 → SessionExpired → login e (error.session_expired)',
    () async {
      final auth = AuthBloc(
        AdminAuthRepository(
          ApiClient(
            Uri.parse('https://api.test'),
            MockClient(
              (_) async => json(200, {
                'name': 'N',
                'email': 'e@x.com',
                'role': 'staff',
                'vendor_id': null,
              }),
            ),
          ),
        ),
      )..add(AuthStarted());
      expect(await auth.stream.first, isA<Authenticated>());
      auth.add(SessionExpired());
      final s = await auth.stream.first as Unauthenticated;
      expect(s.error?.code, 'session_expired');
    },
  );

  test(
    'login er age 401 (bhul password) e SessionExpired kichu kore na',
    () async {
      final auth = AuthBloc(
        AdminAuthRepository(
          ApiClient(
            Uri.parse('https://api.test'),
            MockClient((_) async => err(401, 'unauthenticated')),
          ),
        ),
      )..add(AuthStarted());
      await auth.stream.first;
      auth.add(SessionExpired());
      await Future<void>.delayed(Duration.zero);
      expect((auth.state as Unauthenticated).error, isNull);
    },
  );

  group('menu (11.2, canOpen)', () {
    for (final (role, want, hidden) in [
      (
        'owner',
        [
          '/',
          '/orders',
          '/packing',
          '/shipments',
          '/products',
          '/categories',
          '/inventory',
          '/coupons',
          '/zones',
          '/customers',
          '/staff',
          '/audit',
        ],
        <String>[],
      ),
      (
        'staff',
        ['/', '/orders', '/packing', '/shipments', '/inventory'],
        [
          '/products',
          '/categories',
          '/coupons',
          '/zones',
          '/customers',
          '/staff',
          '/audit',
        ],
      ),
      (
        'vendor_staff',
        ['/', '/orders', '/packing', '/shipments', '/inventory'],
        [
          '/products',
          '/categories',
          '/coupons',
          '/zones',
          '/customers',
          '/staff',
          '/audit',
        ],
      ),
    ]) {
      testWidgets('$role 360px drawer, 1280px rail', (tester) async {
        for (final w in [360.0, 1280.0]) {
          await pumpPage(
            tester,
            AppShell(location: '/orders', child: const SizedBox()),
            role: role,
            size: Size(w, 800),
          );
          expect(tester.takeException(), isNull);
          if (w < 600) {
            await tester.tap(find.byTooltip('Open navigation menu'));
            await tester.pumpAndSettle();
          }
          for (final r in want) {
            expect(
              find.byKey(Key('nav.$r')),
              findsOneWidget,
              reason: '$role $w $r',
            );
          }
          for (final r in hidden) {
            expect(find.byKey(Key('nav.$r')), findsNothing, reason: '$role $r');
          }
        }
      });
    }
    test('menu er protiti route router e ache', () {
      final paths =
          buildRouter(
                AuthBloc(
                  AdminAuthRepository(
                    ApiClient(
                      Uri.parse('https://x'),
                      MockClient((_) async => http.Response('', 500)),
                    ),
                  ),
                ),
              ).configuration.routes
              .expand((r) => r is ShellRoute ? r.routes : [r])
              .whereType<GoRoute>()
              .map((r) => r.path)
              .toSet();
      expect(paths, containsAll(menuRoutes));
    });
  });
}
