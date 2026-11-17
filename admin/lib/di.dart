import 'dart:async';
import 'dart:js_interop';

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
  ..registerSingleton(DashboardRepository(api))
  ..registerSingleton(AdminCatalogRepository(api))
  ..registerSingleton(InventoryRepository(api))
  ..registerSingleton(StaffRepository(api));

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
  pickFile: _pickFile,
);

// Lukano `<input type=file>` (package chhara); `cancel` event e null.
Future<PickedFile?> _pickFile() {
  final done = Completer<PickedFile?>();
  final input = web.HTMLInputElement()
    ..type = 'file'
    ..accept = 'image/jpeg,image/png';
  input.addEventListener(
    'cancel',
    ((web.Event _) {
      if (!done.isCompleted) done.complete(null);
    }).toJS,
  );
  input.addEventListener(
    'change',
    ((web.Event _) {
      final f = input.files?.item(0);
      if (done.isCompleted) return;
      if (f == null) return done.complete(null);
      done.complete(
        f.arrayBuffer().toDart.then(
          (b) => (bytes: b.toDart.asUint8List(), name: f.name),
        ),
      );
    }).toJS,
  );
  input.click();
  return done.future;
}

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
