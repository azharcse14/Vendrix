import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/variant_stock_cubit.dart';
import '../widgets/stock_dialogs.dart';
import 'inventory_page.dart';

/// `/inventory/:variantId`: stock entry, komano, batch, movement. [variant] list theke (`extra`; URL shoja khulle
/// null, header nai). `staff`/`vendor_staff` e kena dam nai (API o dey na, 11.7 item 1).
class VariantStockPage extends StatelessWidget {
  final InventoryVariant? variant;
  const VariantStockPage({super.key, this.variant});

  @override
  Widget build(BuildContext context) {
    final c = context.read<VariantStockCubit>();
    final auth = context.watch<AuthBloc>().state;
    final showCost =
        auth is Authenticated &&
        !{'staff', 'vendor_staff'}.contains(auth.staff.role);
    final text = Theme.of(context).textTheme;
    return Scaffold(
      body: switch (context.watch<VariantStockCubit>().state) {
        VariantStockLoading() => const Center(
          child: CircularProgressIndicator(),
        ),
        VariantStockFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        final VariantStockLoaded s => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (variant != null) ...[
              Text(variantName(context, variant!), style: text.titleMedium),
              Text(variant!.sku),
            ],
            if (s.error != null) ErrorLine(_error(context, s.error!)),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                FilledButton.icon(
                  key: const Key('stock.add'),
                  onPressed: s.busy
                      ? null
                      : () async {
                          final b = await showBatchDialog(
                            context,
                            showCost: showCost,
                          );
                          if (b != null) {
                            await c.addBatch(
                              b.quantity,
                              costPrice: b.costPrice,
                              batchCode: b.batchCode,
                              harvestDate: b.harvestDate,
                              expiryDate: b.expiryDate,
                            );
                          }
                        },
                  icon: const Icon(Icons.add),
                  label: Text(context.t('admin.stock.add')),
                ),
                OutlinedButton.icon(
                  key: const Key('stock.reduce'),
                  onPressed: s.busy
                      ? null
                      : () async {
                          final r = await showReduceDialog(context);
                          if (r != null) {
                            await c.reduce(r.quantity, r.reason, r.note);
                          }
                        },
                  icon: const Icon(Icons.remove),
                  label: Text(context.t('admin.stock.reduce')),
                ),
              ],
            ),
            const SizedBox(height: 16),
            Text(context.t('admin.stock.batches'), style: text.titleSmall),
            if (s.batches.isEmpty) Text(context.t('admin.empty')),
            for (final b in s.batches)
              ListTile(
                key: Key('batch.${b.id}'),
                contentPadding: EdgeInsets.zero,
                title: Text(b.batchCode ?? context.t('admin.stock.no_code')),
                subtitle: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      context.t('admin.stock.remaining', {
                        'left': context.count(b.quantityRemaining),
                        'got': context.count(b.quantityReceived),
                      }),
                    ),
                    if (b.expiryDate != null)
                      Text(
                        context.t('admin.inventory.expiry', {
                          'date': b.expiryDate!,
                        }),
                      ),
                    if (b.costPrice != null)
                      Text(
                        context.t('admin.stock.cost', {
                          'amount': context.taka(b.costPrice!),
                        }),
                        key: Key('batch.${b.id}.cost'),
                      ),
                  ],
                ),
              ),
            const SizedBox(height: 16),
            Text(context.t('admin.stock.movements'), style: text.titleSmall),
            if (s.movements.isEmpty) Text(context.t('admin.empty')),
            for (final m in s.movements)
              ListTile(
                key: Key('movement.${m.id}'),
                contentPadding: EdgeInsets.zero,
                title: Text(
                  '${m.change > 0 ? '+' : ''}${context.count(m.change)} · '
                  '${context.t('admin.reason.${m.reason}')}',
                ),
                subtitle: Text(
                  [context.date(m.createdAt), ?m.note].join(' · '),
                ),
              ),
          ],
        ),
      },
    );
  }

  String _error(BuildContext context, ApiError e) =>
      e.code == 'out_of_stock' && e.details['available'] is int
      ? context.t('admin.stock.out_of_stock', {
          'available': context.count(e.details['available'] as int),
        })
      : context.errorText(e);
}
