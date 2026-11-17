import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/inventory_list_cubit.dart';

/// Variant er naam: product + option (`Chal · 1 kg`).
String variantName(BuildContext context, InventoryVariant v) => [
  context.text(v.productName),
  ...v.variantLabel.values.map(context.text),
].join(' · ');

/// `/inventory`: khoja, low filter, variant list; tap e stock (`extra` e row, header er jonno).
class InventoryPage extends StatelessWidget {
  const InventoryPage({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.read<InventoryListCubit>();
    final s = context.watch<InventoryListCubit>().state;
    final router = GoRouter.maybeOf(context);
    return Scaffold(
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          TextField(
            key: const Key('inventory.search'),
            textInputAction: TextInputAction.search,
            decoration: InputDecoration(
              labelText: context.t('admin.inventory.search'),
              prefixIcon: const Icon(Icons.search),
            ),
            onSubmitted: c.search,
          ),
          const SizedBox(height: 8),
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: FilterChip(
              key: const Key('inventory.low'),
              label: Text(context.t('admin.inventory.low')),
              selected: c.low,
              onSelected: (_) => c.toggleLow(),
            ),
          ),
          ...switch (s) {
            InventoryListLoading() => [
              const Center(child: CircularProgressIndicator()),
            ],
            InventoryListFailed(:final error) => [
              ErrorRetry(error, onRetry: c.load),
            ],
            InventoryListLoaded(:final page) => [
              if (page.items.isEmpty) Text(context.t('admin.empty')),
              for (final v in page.items)
                Card(
                  margin: const EdgeInsets.only(top: 8),
                  child: ListTile(
                    key: Key('variant.${v.variantId}'),
                    title: Text(variantName(context, v)),
                    subtitle: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(v.sku),
                        if (v.nextExpiry != null)
                          Text(
                            context.t('admin.inventory.expiry', {
                              'date': v.nextExpiry!,
                            }),
                          ),
                      ],
                    ),
                    trailing: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      crossAxisAlignment: CrossAxisAlignment.end,
                      children: [
                        Text(
                          context.count(v.stock),
                          style: Theme.of(context).textTheme.titleMedium,
                        ),
                        if (v.stock <= v.lowStockAt)
                          Text(
                            context.t('admin.inventory.low_badge'),
                            key: Key('variant.${v.variantId}.low'),
                            style: TextStyle(
                              color: Theme.of(context).colorScheme.error,
                            ),
                          ),
                      ],
                    ),
                    onTap: () =>
                        router?.go('/inventory/${v.variantId}', extra: v),
                  ),
                ),
              PageBar(
                prefix: 'inventory',
                page: page.page,
                hasNext: page.hasNext,
                onPage: c.load,
              ),
            ],
          },
        ],
      ),
    );
  }
}
