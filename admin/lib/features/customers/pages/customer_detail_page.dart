import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../../../core/widgets/note_dialog.dart';
import '../../orders/widgets/status_chip.dart';
import '../bloc/customer_detail_cubit.dart';

/// `/customers/:id`: profile, block/unblock, shesh 50 order (tap e order detail).
class CustomerDetailPage extends StatelessWidget {
  const CustomerDetailPage({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.read<CustomerDetailCubit>();
    final router = GoRouter.maybeOf(context);
    final text = Theme.of(context).textTheme;
    return Scaffold(
      body: switch (context.watch<CustomerDetailCubit>().state) {
        CustomerDetailLoading() => const Center(
          child: CircularProgressIndicator(),
        ),
        CustomerDetailFailed(:final error) => ErrorRetry(
          error,
          onRetry: c.load,
        ),
        final CustomerDetailLoaded s => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text(s.customer.name ?? s.customer.phone, style: text.titleLarge),
            Text(
              [
                s.customer.phone,
                ?s.customer.email,
                context.t('admin.customer.since', {
                  'date': context.date(s.customer.createdAt),
                }),
                if (s.customer.blocked) context.t('admin.customer.is_blocked'),
              ].join(' · '),
            ),
            const SizedBox(height: 12),
            if (s.error != null) ErrorLine(context.errorText(s.error!)),
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: s.customer.blocked
                  ? OutlinedButton.icon(
                      key: const Key('customer.unblock'),
                      onPressed: s.busy
                          ? null
                          : () async {
                              if (await confirmDialog(
                                context,
                                'admin.customer.unblock_confirm',
                              )) {
                                await c.unblock();
                              }
                            },
                      icon: const Icon(Icons.lock_open),
                      label: Text(context.t('admin.customer.unblock')),
                    )
                  : OutlinedButton.icon(
                      key: const Key('customer.block'),
                      onPressed: s.busy
                          ? null
                          : () async {
                              final reason = await showNoteDialog(
                                context,
                                title: 'admin.customer.block',
                                hint: 'admin.customer.block_hint',
                                required: false,
                                maxLength: 200,
                              );
                              if (reason != null) await c.block(reason);
                            },
                      icon: const Icon(Icons.block),
                      label: Text(context.t('admin.customer.block')),
                    ),
            ),
            const Divider(height: 32),
            Text(context.t('admin.customer.recent'), style: text.titleSmall),
            if (s.customer.orders.isEmpty) Text(context.t('admin.empty')),
            for (final o in s.customer.orders)
              ListTile(
                key: Key('customer.order.${o.id}'),
                contentPadding: EdgeInsets.zero,
                title: Wrap(
                  spacing: 8,
                  crossAxisAlignment: WrapCrossAlignment.center,
                  children: [Text(o.orderNumber), StatusChip(o.displayStatus)],
                ),
                subtitle: Text(context.date(o.placedAt)),
                trailing: Text(context.taka(o.total)),
                onTap: () => router?.go('/orders/${o.id}'),
              ),
          ],
        ),
      },
    );
  }
}
