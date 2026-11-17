import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:shared/shared.dart';

import '../../../core/browser.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/product_form_cubit.dart';
import '../widgets/variant_dialog.dart';

/// 6.7 er `unit`.
const units = ['piece', 'kg', 'gram', 'litre', 'ml', 'dozen', 'pack'];

final _slug = RegExp(r'^[a-z0-9]+(-[a-z0-9]+)*$');

// Khali = chobi na dekhiye icon (local e `https://img.localhost:8443`, website er IMAGE_BASE_URL er moto).
const _imageBase = String.fromEnvironment('IMAGE_BASE_URL');

/// `/products/new`, `/products/:id`: product form, save er por variant ar chobi. SEO/attributes ekhon na (M7b.md).
class ProductFormPage extends StatelessWidget {
  const ProductFormPage({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.read<ProductFormCubit>();
    return Scaffold(
      body: switch (context.watch<ProductFormCubit>().state) {
        ProductFormLoading() => const Center(
          child: CircularProgressIndicator(),
        ),
        ProductFormFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        // Column (ListView na): form er field scroll e harale state/validator jay
        final ProductFormReady s => SingleChildScrollView(
          padding: const EdgeInsets.all(16),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.stretch,
            children: [
              if (s.error != null) ErrorLine(context.errorText(s.error!)),
              _ProductForm(s),
              if (s.product != null) ...[
                const Divider(height: 32),
                _Variants(s),
                const Divider(height: 32),
                _Images(s),
              ],
            ],
          ),
        ),
      },
    );
  }
}

class _ProductForm extends StatefulWidget {
  final ProductFormReady s;
  const _ProductForm(this.s);

  @override
  State<_ProductForm> createState() => _ProductFormState();
}

class _ProductFormState extends State<_ProductForm> {
  final _form = GlobalKey<FormState>();
  late final p = widget.s.product;
  late final _nameBn = TextEditingController(text: p?.name.values['bn']);
  late final _nameEn = TextEditingController(text: p?.name.values['en']);
  late final _slugC = TextEditingController(text: p?.slug);
  late final _descBn = TextEditingController(text: p?.description.values['bn']);
  late final _descEn = TextEditingController(text: p?.description.values['en']);
  late final _keywords = TextEditingController(text: p?.searchKeywords);
  late int? _category = p?.categoryId;
  late var _unit = p?.unit ?? 'piece';
  late var _variable = p?.variableWeight ?? false;
  late var _active = p?.isActive ?? true;

  @override
  void dispose() {
    for (final c in [_nameBn, _nameEn, _slugC, _descBn, _descEn, _keywords]) {
      c.dispose();
    }
    super.dispose();
  }

  Future<void> _save() async {
    if (!_form.currentState!.validate()) return;
    final c = context.read<ProductFormCubit>();
    final router = GoRouter.maybeOf(context);
    final isNew = c.id == null;
    final ok = await c.save({
      'name': {'bn': _nameBn.text.trim(), 'en': _nameEn.text.trim()},
      'slug': _slugC.text.trim(),
      'description': {'bn': _descBn.text.trim(), 'en': _descEn.text.trim()},
      'category_id': _category,
      'unit': _unit,
      'variable_weight': _variable,
      'search_keywords': _keywords.text.trim(),
      'is_active': _active,
    });
    if (ok && isNew) router?.go('/products/${c.id}');
  }

  @override
  Widget build(BuildContext context) {
    final req = context.t('admin.required');
    final badSlug = context.t('admin.slug_invalid');
    Widget field(
      String key,
      TextEditingController c,
      String label, {
      String? Function(String?)? validator,
      int maxLines = 1,
    }) => Padding(
      padding: const EdgeInsets.only(bottom: 12),
      child: TextFormField(
        key: Key('product.$key'),
        controller: c,
        maxLines: maxLines,
        decoration: InputDecoration(labelText: context.t(label)),
        validator: validator,
      ),
    );
    return Form(
      key: _form,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          field(
            'name_bn',
            _nameBn,
            'admin.product.name_bn',
            validator: (v) => (v ?? '').trim().isEmpty ? req : null,
          ),
          field('name_en', _nameEn, 'admin.product.name_en'),
          field(
            'slug',
            _slugC,
            'admin.product.slug',
            validator: (v) {
              final t = (v ?? '').trim();
              return t.length <= 80 && _slug.hasMatch(t) ? null : badSlug;
            },
          ),
          field('desc_bn', _descBn, 'admin.product.desc_bn', maxLines: 4),
          field('desc_en', _descEn, 'admin.product.desc_en', maxLines: 4),
          Padding(
            padding: const EdgeInsets.only(bottom: 12),
            child: DropdownButtonFormField<int?>(
              key: const Key('product.category'),
              isExpanded: true,
              initialValue: _category,
              decoration: InputDecoration(
                labelText: context.t('admin.product.category'),
              ),
              items: [
                DropdownMenuItem(child: Text(context.t('admin.none'))),
                for (final cat in widget.s.categories)
                  DropdownMenuItem(
                    value: cat.id,
                    child: Text(context.text(cat.name)),
                  ),
              ],
              onChanged: (v) => _category = v,
            ),
          ),
          DropdownButtonFormField<String>(
            key: const Key('product.unit'),
            isExpanded: true,
            initialValue: _unit,
            decoration: InputDecoration(
              labelText: context.t('admin.product.unit'),
            ),
            items: [
              for (final u in units)
                DropdownMenuItem(
                  value: u,
                  child: Text(context.t('admin.unit.$u')),
                ),
            ],
            onChanged: (v) => setState(() {
              _unit = v!;
              if (_unit != 'kg') _variable = false; // DB CHECK: shudhu kg
            }),
          ),
          SwitchListTile(
            key: const Key('product.variable_weight'),
            contentPadding: EdgeInsets.zero,
            title: Text(context.t('admin.product.variable_weight')),
            value: _variable,
            onChanged: _unit == 'kg'
                ? (v) => setState(() => _variable = v)
                : null,
          ),
          field('keywords', _keywords, 'admin.product.keywords'),
          SwitchListTile(
            key: const Key('product.active'),
            contentPadding: EdgeInsets.zero,
            title: Text(context.t('admin.active')),
            value: _active,
            onChanged: (v) => setState(() => _active = v),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: AlignmentDirectional.centerStart,
            child: FilledButton(
              key: const Key('product.save'),
              onPressed: widget.s.busy ? null : _save,
              child: Text(context.t('admin.save')),
            ),
          ),
        ],
      ),
    );
  }
}

class _Variants extends StatelessWidget {
  final ProductFormReady s;
  const _Variants(this.s);

  Future<void> _edit(BuildContext context, AdminVariant? v) async {
    final c = context.read<ProductFormCubit>();
    final body = await showVariantDialog(context, v);
    if (body != null) await c.saveVariant(v?.id, body);
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text(
        context.t('admin.product.variants'),
        style: Theme.of(context).textTheme.titleMedium,
      ),
      for (final v in s.product!.variants)
        ListTile(
          key: Key('variant.${v.id}'),
          contentPadding: EdgeInsets.zero,
          title: Text(v.sku),
          subtitle: Text(
            [
              ...v.options.values.map(context.text),
              context.taka(v.price),
              context.t('admin.stock.count', {'n': context.count(v.stock)}),
              if (!v.isActive) context.t('admin.inactive'),
            ].join(' · '),
          ),
          trailing: const Icon(Icons.edit_outlined),
          onTap: s.busy ? null : () => _edit(context, v),
        ),
      TextButton.icon(
        key: const Key('variant.add'),
        onPressed: s.busy ? null : () => _edit(context, null),
        icon: const Icon(Icons.add),
        label: Text(context.t('admin.variant.new')),
      ),
    ],
  );
}

class _Images extends StatelessWidget {
  final ProductFormReady s;
  const _Images(this.s);

  Future<void> _upload(BuildContext context) async {
    final c = context.read<ProductFormCubit>();
    final f = await context.read<Browser>().pickFile();
    if (f != null) await c.uploadImage(f);
  }

  Future<void> _delete(BuildContext context, int id) async {
    final c = context.read<ProductFormCubit>();
    if (await confirmDialog(context, 'admin.image.delete_confirm')) {
      await c.deleteImage(id);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.read<ProductFormCubit>();
    final images = s.product!.images;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(
          context.t('admin.product.images'),
          style: Theme.of(context).textTheme.titleMedium,
        ),
        for (final (i, img) in images.indexed)
          ListTile(
            contentPadding: EdgeInsets.zero,
            leading: SizedBox.square(
              dimension: 48,
              child: _imageBase.isEmpty
                  ? const Icon(Icons.image_outlined)
                  : Image.network(
                      '$_imageBase/${img.storageKey}',
                      fit: BoxFit.cover,
                      // <img> element: CORS ar CSP connect-src chhara (img-src e img domain)
                      webHtmlElementStrategy: WebHtmlElementStrategy.prefer,
                      errorBuilder: (_, _, _) =>
                          const Icon(Icons.broken_image_outlined),
                    ),
            ),
            title: Text(context.text(img.alt)),
            subtitle: Text('${img.width}×${img.height}'),
            trailing: Wrap(
              children: [
                IconButton(
                  key: Key('image.${img.id}.up'),
                  tooltip: context.t('admin.image.up'),
                  onPressed: s.busy || i == 0 ? null : () => c.moveImage(i, -1),
                  icon: const Icon(Icons.arrow_upward),
                ),
                IconButton(
                  key: Key('image.${img.id}.down'),
                  tooltip: context.t('admin.image.down'),
                  onPressed: s.busy || i == images.length - 1
                      ? null
                      : () => c.moveImage(i, 1),
                  icon: const Icon(Icons.arrow_downward),
                ),
                IconButton(
                  key: Key('image.${img.id}.delete'),
                  tooltip: context.t('admin.delete'),
                  onPressed: s.busy ? null : () => _delete(context, img.id),
                  icon: const Icon(Icons.delete_outline),
                ),
              ],
            ),
          ),
        Text(context.t('admin.image.hint')),
        TextButton.icon(
          key: const Key('image.upload'),
          onPressed: s.busy || images.length >= 10
              ? null
              : () => _upload(context),
          icon: const Icon(Icons.upload),
          label: Text(context.t('admin.image.upload')),
        ),
      ],
    );
  }
}
