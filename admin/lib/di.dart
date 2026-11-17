import 'package:flutter/widgets.dart';
import 'package:get_it/get_it.dart';
import 'package:go_router/go_router.dart';
import 'package:http/browser_client.dart';
import 'package:shared/shared.dart';

import 'app.dart';
import 'core/auth/auth_bloc.dart';
import 'router.dart';

final getIt = GetIt.instance;

// Khali = same-origin (local admin.localhost, 13.1); production e https://api.<domain>.
const _apiBase = String.fromEnvironment('API_BASE_URL');

/// Shob register (11.3 item 1), session check er por app (redirect kokhono AuthUnknown dekhe na).
Future<Widget> bootstrap() async {
  // Cookie `__Host-asid` browser rakhe; Dart token dekhe na (11.3 item 4).
  final api = ApiClient(
    _apiBase.isEmpty ? Uri.base.resolve('/') : Uri.parse(_apiBase),
    BrowserClient()..withCredentials = true,
  );
  getIt
    ..registerSingleton(api)
    ..registerSingleton(AdminAuthRepository(api))
    ..registerSingleton(AuthBloc(getIt<AdminAuthRepository>()));
  getIt.registerSingleton<GoRouter>(buildRouter(getIt<AuthBloc>()));
  final auth = getIt<AuthBloc>()..add(AuthStarted());
  await auth.stream.first;
  return AdminApp(auth: auth, router: getIt<GoRouter>());
}
