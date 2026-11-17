import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/product_list_cubit.dart';

/// `/products`: list, notun, tap e form.
class ProductListPage extends StatelessWidget {
  const ProductListPage({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.read<ProductListCubit>();
    final s = context.watch<ProductListCubit>().state;
    final router = GoRouter.maybeOf(context);
    return Scaffold(
      body: switch (s) {
        ProductListLoading() => const Center(
          child: CircularProgressIndicator(),
        ),
        ProductListFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        ProductListLoaded(:final page) => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: FilledButton.icon(
                key: const Key('products.new'),
                onPressed: () => router?.go('/products/new'),
                icon: const Icon(Icons.add),
                label: Text(context.t('admin.products.new')),
              ),
            ),
            if (page.items.isEmpty) Text(context.t('admin.empty')),
            for (final p in page.items)
              Card(
                margin: const EdgeInsets.only(top: 8),
                child: ListTile(
                  key: Key('product.${p.id}'),
                  title: Text(context.text(p.name)),
                  subtitle: Text(p.slug),
                  trailing: p.isActive
                      ? null
                      : Chip(
                          key: Key('product.${p.id}.inactive'),
                          label: Text(context.t('admin.inactive')),
                        ),
                  onTap: () => router?.go('/products/${p.id}'),
                ),
              ),
            PageBar(
              prefix: 'products',
              page: page.page,
              hasNext: page.hasNext,
              onPage: c.load,
            ),
          ],
        ),
      },
    );
  }
}
