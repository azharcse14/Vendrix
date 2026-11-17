import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/pick_list_cubit.dart';

/// `/packing` (8.11 item 1, 11.4): ajker pick list (variant er mot, order), print, order dhore pack.
class PackingPage extends StatelessWidget {
  const PackingPage({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.read<PickListCubit>();
    final s = context.watch<PickListCubit>().state;
    final text = Theme.of(context).textTheme;
    return Scaffold(
      body: switch (s) {
        PickListLoading() => const Center(child: CircularProgressIndicator()),
        PickListFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        PickListLoaded(:final list, :final error) => RefreshIndicator(
          onRefresh: c.load,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              Row(
                children: [
                  Expanded(child: Text(list.date, style: text.titleMedium)),
                  OutlinedButton.icon(
                    key: const Key('pick.print'),
                    onPressed: c.print,
                    icon: const Icon(Icons.print),
                    label: Text(context.t('print.pick_list')),
                  ),
                ],
              ),
              if (error != null)
                Text(
                  context.errorText(error),
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              if (list.fulfillments.isEmpty)
                Padding(
                  padding: const EdgeInsets.all(16),
                  child: Text(context.t('print.empty')),
                )
              else ...[
                const SizedBox(height: 8),
                Text(context.t('print.total'), style: text.titleSmall),
                for (final v in list.variants) _VariantLine(v),
                const SizedBox(height: 8),
                Text(context.t('print.orders'), style: text.titleSmall),
                for (final f in list.fulfillments)
                  Card(
                    child: ListTile(
                      title: Text(f.orderNumber),
                      subtitle: Text(
                        '${f.recipientName} · ${f.area}, ${f.district}',
                      ),
                      trailing: FilledButton(
                        key: Key('pick.${f.id}.pack'),
                        onPressed: () =>
                            context.go('/packing/${f.orderId}/${f.id}'),
                        child: Text(context.t('admin.fulfillment.pack')),
                      ),
                    ),
                  ),
              ],
            ],
          ),
        ),
      },
    );
  }
}

class _VariantLine extends StatelessWidget {
  final PickVariant v;
  const _VariantLine(this.v);

  @override
  Widget build(BuildContext context) => ListTile(
    dense: true,
    contentPadding: EdgeInsets.zero,
    title: Text(
      '${context.text(v.productName)} (${context.text(v.variantLabel)})',
    ),
    subtitle: Text(v.sku),
    trailing: Text(
      [
        '× ${context.count(v.quantity)}',
        if (v.orderedWeightG case final g?)
          context.t('print.grams', {'n': context.count(g)}),
      ].join(' · '),
    ),
  );
}
