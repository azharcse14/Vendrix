import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:go_router/go_router.dart';

import 'core/auth/auth_bloc.dart';
import 'core/l10n/locale_cubit.dart';
import 'core/theme/theme_cubit.dart';

/// App er gora: app-wide bloc (11.3) ar router.
class AdminApp extends StatelessWidget {
  final AuthBloc auth;
  final GoRouter router;
  const AdminApp({super.key, required this.auth, required this.router});

  @override
  Widget build(BuildContext context) => MultiBlocProvider(
    providers: [
      BlocProvider.value(value: auth),
      BlocProvider(create: (_) => ThemeCubit()),
      BlocProvider(create: (_) => LocaleCubit()),
    ],
    child: Builder(
      builder: (context) => MaterialApp.router(
        onGenerateTitle: (context) => context.t('admin.title'),
        theme: context.watch<ThemeCubit>().state,
        locale: Locale(context.watch<LocaleCubit>().state),
        supportedLocales: const [Locale('bn'), Locale('en')],
        localizationsDelegates: GlobalMaterialLocalizations.delegates,
        routerConfig: router,
        debugShowCheckedModeBanner: false,
      ),
    ),
  );
}
