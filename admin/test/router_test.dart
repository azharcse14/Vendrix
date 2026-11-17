import 'dart:convert';

import 'package:admin/app.dart';
import 'package:admin/core/auth/auth_bloc.dart';
import 'package:admin/core/browser.dart';
import 'package:admin/core/locator.dart';
import 'package:admin/features/auth/pages/login_page.dart';
import 'package:admin/features/auth/pages/totp_page.dart';
import 'package:admin/features/dashboard/pages/dashboard_page.dart';
import 'package:admin/router.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';

StaffProfile profile(String role) => StaffProfile.fromJson({
  'name': 'N',
  'email': 'n@example.com',
  'role': role,
  'vendor_id': null,
});

void main() {
  group('authRedirect (11.3)', () {
    final owner = Authenticated(profile('owner'));
    final staff = Authenticated(profile('staff'));
    final setup = SetupRequired(SetupStep.password);
    final totp = TotpRequired('tt');
    final out = Unauthenticated();

    final cases = <(AuthState, String, String?)>[
      (out, '/', '/login'),
      (out, '/orders/5', '/login'),
      (out, '/login', null),
      (out, '/login/totp', '/login'),
      (out, '/login/setup', '/login'),
      (totp, '/login/totp', null),
      (totp, '/', '/login/totp'),
      (totp, '/login', '/login/totp'),
      (setup, '/login/setup', null),
      (setup, '/', '/login/setup'),
      (setup, '/login', '/login/setup'),
      (owner, '/login', '/'),
      (owner, '/login/setup', '/'),
      (owner, '/', null),
      (owner, '/coupons', null),
      (staff, '/coupons', '/'), // role nai → dashboard (11.2)
      (staff, '/orders', null),
      (owner, '/nai', '/'),
    ];
    for (final (s, loc, want) in cases) {
      test('${s.runtimeType} $loc → $want', () {
        expect(authRedirect(s, loc), want);
      });
    }
  });

  testWidgets('AuthBloc bodlale router nijei redirect kore', (tester) async {
    final client = MockClient((req) async {
      final body = switch (req.url.path) {
        '/api/admin/auth/login' => {'totp_required': true, 'temp_token': 'tt'},
        '/api/admin/auth/totp' => {'setup_required': false},
        '/api/admin/me' => {
          'name': 'N',
          'email': 'n@example.com',
          'role': 'owner',
          'vendor_id': null,
        },
        _ => null,
      };
      return http.Response(jsonEncode(body ?? {}), body == null ? 404 : 200);
    });
    final api = ApiClient(Uri.parse('https://api.test'), client);
    // dashboard er cubit getIt theke (di.dart er registerServices er moto)
    getIt
      ..registerSingleton(AdminOrderRepository(api))
      ..registerSingleton(DashboardRepository(api))
      ..registerSingleton(
        Browser(openTab: (_) {}, open: (_) {}, pickFile: () async => null),
      );
    addTearDown(getIt.reset);
    final auth = AuthBloc(AdminAuthRepository(api));
    final router = buildRouter(auth);
    await tester.pumpWidget(AdminApp(auth: auth, router: router));
    await tester.pumpAndSettle();
    expect(find.byType(LoginPage), findsOneWidget);

    auth.add(LoginSubmitted('s', 'e@x.com', 'pw'));
    await tester.pumpAndSettle();
    expect(find.byType(TotpPage), findsOneWidget);

    auth.add(MfaSubmitted('123456'));
    await tester.pumpAndSettle();
    expect(find.byType(DashboardPage), findsOneWidget);
    expect(router.state.matchedLocation, '/');
  });
}
