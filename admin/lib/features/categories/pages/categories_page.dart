import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/categories_cubit.dart';
import '../widgets/category_dialog.dart';

/// `/categories`: tree (indent), notun, edit, delete (confirm).
class CategoriesPage extends StatelessWidget {
  const CategoriesPage({super.key});

  Future<void> _edit(
    BuildContext context,
    List<AdminCategory> all,
    AdminCategory? cat,
  ) async {
    final c = context.read<CategoriesCubit>();
    final body = await showCategoryDialog(context, all, cat);
    if (body != null) await c.save(cat?.id, body);
  }

  Future<void> _delete(BuildContext context, int id) async {
    final c = context.read<CategoriesCubit>();
    if (await confirmDialog(context, 'admin.category.delete_confirm')) {
      await c.delete(id);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.read<CategoriesCubit>();
    return Scaffold(
      body: switch (context.watch<CategoriesCubit>().state) {
        CategoriesLoading() => const Center(child: CircularProgressIndicator()),
        CategoriesFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        final CategoriesLoaded s => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (s.error != null) ErrorLine(context.errorText(s.error!)),
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: FilledButton.icon(
                key: const Key('categories.new'),
                onPressed: s.busy ? null : () => _edit(context, s.all, null),
                icon: const Icon(Icons.add),
                label: Text(context.t('admin.category.new')),
              ),
            ),
            if (s.all.isEmpty) Text(context.t('admin.empty')),
            for (final (:category, :depth) in categoryTree(s.all))
              ListTile(
                key: Key('category.${category.id}'),
                contentPadding: EdgeInsetsDirectional.only(
                  start: 8 + 24.0 * depth,
                ),
                title: Text(context.text(category.name)),
                subtitle: Text(
                  category.isActive
                      ? category.slug
                      : '${category.slug} · ${context.t('admin.inactive')}',
                ),
                trailing: Wrap(
                  children: [
                    IconButton(
                      key: Key('category.${category.id}.edit'),
                      tooltip: context.t('admin.edit'),
                      onPressed: s.busy
                          ? null
                          : () => _edit(context, s.all, category),
                      icon: const Icon(Icons.edit_outlined),
                    ),
                    IconButton(
                      key: Key('category.${category.id}.delete'),
                      tooltip: context.t('admin.delete'),
                      onPressed: s.busy
                          ? null
                          : () => _delete(context, category.id),
                      icon: const Icon(Icons.delete_outline),
                    ),
                  ],
                ),
              ),
          ],
        ),
      },
    );
  }
}
