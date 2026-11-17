import 'package:flutter/material.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../bloc/categories_cubit.dart';

final _slug = RegExp(r'^[a-z0-9]+(-[a-z0-9]+)*$');

/// Category form (7.7): body ba null. Parent list e nije ar nijer child nai (cycle server o atkay). Chobi M8 e.
Future<Map<String, Object?>?> showCategoryDialog(
  BuildContext context,
  List<AdminCategory> all,
  AdminCategory? cat,
) => showDialog<Map<String, Object?>>(
  context: context,
  builder: (_) => _CategoryDialog(all, cat),
);

class _CategoryDialog extends StatefulWidget {
  final List<AdminCategory> all;
  final AdminCategory? cat;
  const _CategoryDialog(this.all, this.cat);

  @override
  State<_CategoryDialog> createState() => _CategoryDialogState();
}

class _CategoryDialogState extends State<_CategoryDialog> {
  final _form = GlobalKey<FormState>();
  late final _bn = TextEditingController(text: widget.cat?.name.values['bn']);
  late final _en = TextEditingController(text: widget.cat?.name.values['en']);
  late final _slugC = TextEditingController(text: widget.cat?.slug);
  late final _sort = TextEditingController(
    text: '${widget.cat?.sortOrder ?? 0}',
  );
  late int? _parent = widget.cat?.parentId;
  late var _active = widget.cat?.isActive ?? true;

  @override
  void dispose() {
    for (final c in [_bn, _en, _slugC, _sort]) {
      c.dispose();
    }
    super.dispose();
  }

  void _submit() {
    if (!_form.currentState!.validate()) return;
    Navigator.of(context).pop({
      'name': {'bn': _bn.text.trim(), 'en': _en.text.trim()},
      'slug': _slugC.text.trim(),
      'parent_id': _parent,
      'sort_order': int.parse(_sort.text.trim()),
      'is_active': _active,
    });
  }

  @override
  Widget build(BuildContext context) {
    final req = context.t('admin.required');
    final badSlug = context.t('admin.slug_invalid');
    final badNumber = context.t('admin.number_invalid');
    final cat = widget.cat;
    final exclude = cat == null ? <int>{} : subtree(widget.all, cat.id);
    return AlertDialog(
      title: Text(
        context.t(cat == null ? 'admin.category.new' : 'admin.category.edit'),
      ),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                key: const Key('category.name_bn'),
                controller: _bn,
                decoration: InputDecoration(
                  labelText: context.t('admin.product.name_bn'),
                ),
                validator: (v) => (v ?? '').trim().isEmpty ? req : null,
              ),
              TextFormField(
                key: const Key('category.name_en'),
                controller: _en,
                decoration: InputDecoration(
                  labelText: context.t('admin.product.name_en'),
                ),
              ),
              TextFormField(
                key: const Key('category.slug'),
                controller: _slugC,
                decoration: InputDecoration(
                  labelText: context.t('admin.product.slug'),
                ),
                validator: (v) {
                  final t = (v ?? '').trim();
                  return t.length <= 80 && _slug.hasMatch(t) ? null : badSlug;
                },
              ),
              DropdownButtonFormField<int?>(
                key: const Key('category.parent'),
                isExpanded: true,
                initialValue: _parent,
                decoration: InputDecoration(
                  labelText: context.t('admin.category.parent'),
                ),
                items: [
                  DropdownMenuItem(
                    key: const Key('parent.none'),
                    child: Text(context.t('admin.none')),
                  ),
                  for (final (:category, :depth) in categoryTree(widget.all))
                    if (!exclude.contains(category.id))
                      DropdownMenuItem(
                        key: Key('parent.${category.id}'),
                        value: category.id,
                        child: Text(
                          '${'  ' * depth}${context.text(category.name)}',
                        ),
                      ),
                ],
                onChanged: (v) => _parent = v,
              ),
              TextFormField(
                key: const Key('category.sort'),
                controller: _sort,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(
                  labelText: context.t('admin.sort_order'),
                ),
                validator: (v) {
                  final n = int.tryParse((v ?? '').trim());
                  return n != null && n.abs() <= 2147483647 ? null : badNumber;
                },
              ),
              SwitchListTile(
                key: const Key('category.active'),
                contentPadding: EdgeInsets.zero,
                title: Text(context.t('admin.active')),
                value: _active,
                onChanged: (v) => setState(() => _active = v),
              ),
            ],
          ),
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(context.t('admin.close')),
        ),
        FilledButton(
          key: const Key('category.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
