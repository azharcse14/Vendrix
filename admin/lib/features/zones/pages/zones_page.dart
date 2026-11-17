import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/zones_cubit.dart';
import '../widgets/zone_dialog.dart';

/// `/zones`: delivery zone list (inactive o), notun, edit.
class ZonesPage extends StatelessWidget {
  const ZonesPage({super.key});

  Future<void> _edit(BuildContext context, Zone? z) async {
    final c = context.read<ZonesCubit>();
    final body = await showZoneDialog(context, z);
    if (body != null) await c.save(z?.id, body);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.read<ZonesCubit>();
    return Scaffold(
      body: switch (context.watch<ZonesCubit>().state) {
        ZonesLoading() => const Center(child: CircularProgressIndicator()),
        ZonesFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        final ZonesLoaded s => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (s.error != null) ErrorLine(context.errorText(s.error!)),
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: FilledButton.icon(
                key: const Key('zone.new'),
                onPressed: s.busy ? null : () => _edit(context, null),
                icon: const Icon(Icons.add),
                label: Text(context.t('admin.zone.new')),
              ),
            ),
            if (s.page.items.isEmpty) Text(context.t('admin.empty')),
            for (final z in s.page.items)
              Card(
                key: Key('zone.${z.id}'),
                margin: const EdgeInsets.only(top: 8),
                child: ListTile(
                  title: Wrap(
                    spacing: 8,
                    children: [
                      Text(context.text(z.name)),
                      if (!z.isActive)
                        Text(
                          context.t('admin.inactive'),
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.error,
                          ),
                        ),
                    ],
                  ),
                  subtitle: Text(
                    [
                      '${z.district}, ${z.area ?? context.t('admin.zone.whole_district')}',
                      context.t('admin.zone.charge_is', {
                        'amount': context.taka(z.charge),
                      }),
                      if (z.minOrder > 0)
                        context.t('admin.zone.min_order_is', {
                          'amount': context.taka(z.minOrder),
                        }),
                      if (z.freeDeliveryOver != null)
                        context.t('admin.zone.free_over_is', {
                          'amount': context.taka(z.freeDeliveryOver!),
                        }),
                      context.t(
                        z.sameCity
                            ? 'admin.zone.same_city'
                            : 'admin.zone.outside',
                      ),
                    ].join(' · '),
                  ),
                  trailing: IconButton(
                    key: Key('zone.${z.id}.edit'),
                    tooltip: context.t('admin.edit'),
                    onPressed: s.busy ? null : () => _edit(context, z),
                    icon: const Icon(Icons.edit_outlined),
                  ),
                ),
              ),
            PageBar(
              prefix: 'zones',
              page: s.page.page,
              hasNext: s.page.hasNext,
              onPage: c.load,
            ),
          ],
        ),
      },
    );
  }
}
