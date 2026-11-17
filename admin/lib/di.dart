import 'package:flutter/widgets.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:http/browser_client.dart';
import 'package:shared/shared.dart';
import 'package:web/web.dart' as web;

import 'app.dart';
import 'core/auth/auth_bloc.dart';
import 'core/auth/session_client.dart';
import 'core/browser.dart';
import 'core/locator.dart';
import 'router.dart';

export 'core/locator.dart';

// Khali = same-origin (local admin.localhost, 13.1); production e https://api.<domain>.
const _apiBase = String.fromEnvironment('API_BASE_URL');

/// Repository ar [Browser] register (app ar test e ekoi).
void registerServices(ApiClient api, Browser browser) => getIt
  ..registerSingleton(api)
  ..registerSingleton(browser)
  ..registerSingleton(AdminAuthRepository(api))
  ..registerSingleton(AdminOrderRepository(api))
  ..registerSingleton(DashboardRepository(api));

final _browser = Browser(
  openTab: (url) {
    // tap er bhetore-i khola (Safari async window.open atkay); noopener dile handle null, tai hate
    final w = web.window.open('', '_blank');
    w?.opener = null;
    url.then(
      (u) => w?.location.href = u,
      onError: (Object _) {
        w?.close();
        return '';
      },
    );
  },
  open: (u) => web.window.location.href = u,
);

/// Shob register (11.3 item 1), session check er por app (redirect kokhono AuthUnknown dekhe na).
Future<Widget> bootstrap() async {
  late final AuthBloc auth;
  // Cookie `__Host-asid` browser rakhe; Dart token dekhe na (11.3 item 4). Screen er 401 → login.
  final api = ApiClient(
    _apiBase.isEmpty ? Uri.base.resolve('/') : Uri.parse(_apiBase),
    SessionClient(
      BrowserClient()..withCredentials = true,
      () => auth.add(SessionExpired()),
    ),
  );
  registerServices(api, _browser);
  auth = AuthBloc(getIt<AdminAuthRepository>());
  getIt
    ..registerSingleton(auth)
    ..registerSingleton<GoRouter>(buildRouter(auth));
  auth.add(AuthStarted());
  await auth.stream.first;
  return RepositoryProvider.value(
    value: _browser,
    child: AdminApp(auth: auth, router: getIt<GoRouter>()),
  );
}
