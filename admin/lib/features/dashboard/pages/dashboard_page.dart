import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/dashboard_cubit.dart';

/// `/`: ajker count; tap e oi list (order tab, packing).
class DashboardPage extends StatelessWidget {
  const DashboardPage({super.key});

  @override
  Widget build(BuildContext context) {
    final s = context.watch<DashboardCubit>().state;
    return Scaffold(
      body: switch (s) {
        DashboardLoading() => const Center(child: CircularProgressIndicator()),
        DashboardFailed(:final error) => ErrorRetry(
          error,
          onRetry: context.read<DashboardCubit>().load,
        ),
        DashboardLoaded(:final data) => RefreshIndicator(
          onRefresh: context.read<DashboardCubit>().load,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  for (final (key, value, to) in [
                    ('orders', context.count(data.orders), null),
                    ('sales', context.taka(data.sales), null),
                    (
                      'pending',
                      context.count(data.pending),
                      '/orders?tab=awaiting_confirm',
                    ),
                    ('to_pack', context.count(data.toPack), '/packing'),
                    (
                      'deadline_soon',
                      context.count(data.deadlineSoon),
                      '/orders?tab=deadline_soon',
                    ),
                    ('low_stock', context.count(data.lowStock), null),
                    ('expiring', context.count(data.expiring), null),
                  ])
                    SizedBox(
                      width: 156,
                      child: Card(
                        key: Key('dashboard.$key'),
                        clipBehavior: Clip.antiAlias,
                        child: InkWell(
                          onTap: to == null ? null : () => context.go(to),
                          child: Padding(
                            padding: const EdgeInsets.all(12),
                            child: Column(
                              crossAxisAlignment: CrossAxisAlignment.start,
                              children: [
                                Text(
                                  value,
                                  style: Theme.of(context)
                                      .textTheme
                                      .headlineSmall,
                                ),
                                Text(context.t('admin.dashboard.$key')),
                              ],
                            ),
                          ),
                        ),
                      ),
                    ),
                ],
              ),
            ],
          ),
        ),
      },
    );
  }
}
