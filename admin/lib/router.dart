import 'dart:async';

import 'package:flutter/material.dart';
import 'package:go_router/go_router.dart';

import 'core/auth/auth_bloc.dart';
import 'core/l10n/locale_cubit.dart';
import 'core/permissions.dart';
import 'features/auth/pages/login_page.dart';
import 'features/auth/pages/setup_page.dart';
import 'features/auth/pages/totp_page.dart';
import 'features/dashboard/pages/dashboard_page.dart';

/// Stream e notun value ele router abar redirect dekhe (11.3).
class GoRouterRefreshStream extends ChangeNotifier {
  late final StreamSubscription<Object?> _sub;
  GoRouterRefreshStream(Stream<Object?> stream) {
    _sub = stream.listen((_) => notifyListeners());
  }

  @override
  void dispose() {
    _sub.cancel();
    super.dispose();
  }
}

/// Setup, login ar role er guard (11.3); null = jekhane ache sekhanei.
String? authRedirect(AuthState s, String loc) {
  if (s is SetupRequired) return loc == '/login/setup' ? null : '/login/setup';
  if (s is TotpRequired) return loc == '/login/totp' ? null : '/login/totp';
  if (s is! Authenticated) return loc == '/login' ? null : '/login';
  if (loc.startsWith('/login')) return '/';
  if (!s.staff.canOpen(loc)) return '/';
  return null;
}

/// 11.1 er route (ekhon login ar dashboard; baki screen M7b er porer step e).
GoRouter buildRouter(AuthBloc auth) => GoRouter(
  refreshListenable: GoRouterRefreshStream(auth.stream),
  redirect: (context, state) => authRedirect(auth.state, state.matchedLocation),
  routes: [
    GoRoute(path: '/', builder: (_, _) => const DashboardPage()),
    GoRoute(path: '/login', builder: (_, _) => const LoginPage()),
    GoRoute(path: '/login/totp', builder: (_, _) => const TotpPage()),
    GoRoute(path: '/login/setup', builder: (_, _) => const SetupPage()),
  ],
  errorBuilder: (context, _) => Scaffold(
    body: Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(context.t('admin.not_found')),
          TextButton(
            onPressed: () => context.go('/'),
            child: Text(context.t('admin.home')),
          ),
        ],
      ),
    ),
  ),
);
