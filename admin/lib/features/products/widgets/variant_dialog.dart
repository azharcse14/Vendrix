import 'package:flutter/material.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';

/// Notun variant e option er key (label ekta, bn/en; key user lekhe na).
const defaultOptionKey = 'size';

/// Taka (poisha) → form er lekha (`120.5` na, `120.50`; purno hole `120`).
String takaInput(int? p) => p == null
    ? ''
    : p % 100 == 0
    ? '${p ~/ 100}'
    : '${p ~/ 100}.${'${p % 100}'.padLeft(2, '0')}';

/// Variant form (7.7): body ba bondho korle null. Taka form e taka, body te poisha ([parseTaka]); stock na (8.12).
Future<Map<String, Object?>?> showVariantDialog(
  BuildContext context,
  AdminVariant? v,
) => showDialog<Map<String, Object?>>(
  context: context,
  builder: (_) => _VariantDialog(v),
);

class _VariantDialog extends StatefulWidget {
  final AdminVariant? v;
  const _VariantDialog(this.v);

  @override
  State<_VariantDialog> createState() => _VariantDialogState();
}

class _VariantDialogState extends State<_VariantDialog> {
  final _form = GlobalKey<FormState>();
  late final _key = widget.v?.options.keys.firstOrNull ?? defaultOptionKey;
  late final _sku = TextEditingController(text: widget.v?.sku);
  late final _bn = TextEditingController(
    text: widget.v?.options[_key]?.values['bn'],
  );
  late final _en = TextEditingController(
    text: widget.v?.options[_key]?.values['en'],
  );
  late final _price = TextEditingController(text: takaInput(widget.v?.price));
  late final _compare = TextEditingController(
    text: takaInput(widget.v?.compareAtPrice),
  );
  late final _cost = TextEditingController(
    text: takaInput(widget.v?.costPrice),
  );
  late final _weight = TextEditingController(
    text: widget.v?.nominalWeightG?.toString(),
  );
  late var _active = widget.v?.isActive ?? true;

  @override
  void dispose() {
    for (final c in [_sku, _bn, _en, _price, _compare, _cost, _weight]) {
      c.dispose();
    }
    super.dispose();
  }

  int? _opt(TextEditingController c) =>
      c.text.trim().isEmpty ? null : parseTaka(c.text);

  void _submit() {
    if (!_form.currentState!.validate()) return;
    final bn = _bn.text.trim(), en = _en.text.trim();
    Navigator.of(context).pop({
      'sku': _sku.text.trim(),
      'options': {
        ...?widget.v?.options.map((k, l) => MapEntry(k, l.toJson())),
        if (bn.isNotEmpty || en.isNotEmpty) _key: {'bn': bn, 'en': en},
      },
      'price': parseTaka(_price.text),
      'compare_at_price': _opt(_compare),
      'cost_price': _opt(_cost),
      'nominal_weight_g': int.tryParse(_weight.text.trim()),
      'is_active': _active,
    });
  }

  @override
  Widget build(BuildContext context) {
    // validator callback e context.t na (watch)
    final req = context.t('admin.required');
    final badTaka = context.t('admin.taka_invalid');
    final badCompare = context.t('admin.variant.compare_invalid');
    final badWeight = context.t('admin.variant.weight_invalid');
    String? optTaka(String? v) =>
        (v ?? '').trim().isEmpty || parseTaka(v!) != null ? null : badTaka;
    Widget field(
      String key,
      TextEditingController c,
      String label, {
      String? Function(String?)? validator,
      TextInputType? keyboard,
      int? maxLength,
    }) => Padding(
      padding: const EdgeInsets.only(bottom: 8),
      child: TextFormField(
        key: Key('variant.$key'),
        controller: c,
        keyboardType: keyboard,
        maxLength: maxLength,
        decoration: InputDecoration(labelText: context.t(label)),
        validator: validator,
      ),
    );
    const money = TextInputType.numberWithOptions(decimal: true);
    return AlertDialog(
      title: Text(
        context.t(
          widget.v == null ? 'admin.variant.new' : 'admin.variant.edit',
        ),
      ),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              field(
                'sku',
                _sku,
                'admin.variant.sku',
                maxLength: 64,
                validator: (v) => (v ?? '').trim().isEmpty ? req : null,
              ),
              field('label_bn', _bn, 'admin.variant.label_bn'),
              field('label_en', _en, 'admin.variant.label_en'),
              field(
                'price',
                _price,
                'admin.variant.price',
                keyboard: money,
                validator: (v) => parseTaka(v ?? '') == null ? badTaka : null,
              ),
              field(
                'compare_at',
                _compare,
                'admin.variant.compare_at',
                keyboard: money,
                validator: (v) {
                  final e = optTaka(v);
                  if (e != null || (v ?? '').trim().isEmpty) return e;
                  final p = parseTaka(_price.text);
                  return p != null && parseTaka(v!)! <= p ? badCompare : null;
                },
              ),
              field(
                'cost',
                _cost,
                'admin.variant.cost',
                keyboard: money,
                validator: optTaka,
              ),
              field(
                'weight',
                _weight,
                'admin.variant.weight',
                keyboard: TextInputType.number,
                validator: (v) {
                  final t = (v ?? '').trim();
                  final n = int.tryParse(t);
                  return t.isEmpty || (n != null && n > 0 && n <= 2147483647)
                      ? null
                      : badWeight;
                },
              ),
              SwitchListTile(
                key: const Key('variant.active'),
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
          key: const Key('variant.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
