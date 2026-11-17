import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';

/// `/`: ekhon shudhu shagotom ar logout; count ar menu M7b step 2 e.
class DashboardPage extends StatelessWidget {
  const DashboardPage({super.key});

  @override
  Widget build(BuildContext context) {
    final s = context.watch<AuthBloc>().state;
    final name = s is Authenticated ? s.staff.name : '';
    return Scaffold(
      appBar: AppBar(
        title: Text(context.t('admin.dashboard.title')),
        actions: [
          TextButton(
            onPressed: context.read<LocaleCubit>().toggle,
            child: Text(context.t('admin.language')),
          ),
          TextButton(
            onPressed: () => context.read<AuthBloc>().add(LogoutRequested()),
            child: Text(context.t('admin.logout')),
          ),
        ],
      ),
      body: Padding(
        padding: const EdgeInsets.all(16),
        child: Text(context.t('admin.dashboard.welcome', {'name': name})),
      ),
    );
  }
}
