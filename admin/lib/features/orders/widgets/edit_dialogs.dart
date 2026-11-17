import 'package:flutter/material.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';

String _itemName(BuildContext context, AdminItem i) =>
    '${context.text(i.productName)} (${context.text(i.variantLabel)})';

/// Order edit (11.7 item 7): `open` fulfillment er active item er notun qty (shudhu kom, `0` = bad) ar note;
/// shudhu bodla item. Bondho korle null.
Future<(Map<int, int>, String)?> showEditDialog(
  BuildContext context,
  OrderDetail o,
) => showDialog<(Map<int, int>, String)>(
  context: context,
  builder: (_) => _EditDialog([
    for (final f in o.fulfillments)
      if (f.status == 'open')
        for (final i in f.items)
          if (i.status == 'active') i,
  ]),
);

class _EditDialog extends StatefulWidget {
  final List<AdminItem> items;
  const _EditDialog(this.items);

  @override
  State<_EditDialog> createState() => _EditDialogState();
}

class _EditDialogState extends State<_EditDialog> {
  final _form = GlobalKey<FormState>();
  late final _qty = {
    for (final i in widget.items)
      i.id: TextEditingController(text: '${i.quantity}'),
  };
  final _note = TextEditingController();
  var _noChange = false;

  @override
  void dispose() {
    for (final c in [..._qty.values, _note]) {
      c.dispose();
    }
    super.dispose();
  }

  void _submit() {
    if (!_form.currentState!.validate()) return;
    final changed = {
      for (final i in widget.items)
        if (int.parse(_qty[i.id]!.text.trim()) case final q when q < i.quantity)
          i.id: q,
    };
    if (changed.isEmpty) return setState(() => _noChange = true);
    Navigator.of(context).pop((changed, _note.text.trim()));
  }

  @override
  Widget build(BuildContext context) {
    final req = context.t('admin.required');
    final badQty = context.t('admin.edit.qty_invalid');
    return AlertDialog(
      title: Text(context.t('admin.order.edit')),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(context.t('admin.edit.hint')),
              for (final i in widget.items)
                TextFormField(
                  key: Key('edit.${i.id}'),
                  controller: _qty[i.id],
                  keyboardType: TextInputType.number,
                  decoration: InputDecoration(
                    labelText: _itemName(context, i),
                    helperText: context.t('admin.edit.was', {
                      'n': context.count(i.quantity),
                    }),
                  ),
                  validator: (v) {
                    final n = int.tryParse((v ?? '').trim());
                    return n != null && n >= 0 && n <= i.quantity
                        ? null
                        : badQty;
                  },
                ),
              TextFormField(
                key: const Key('edit.note'),
                controller: _note,
                maxLength: 500,
                decoration: InputDecoration(labelText: context.t('admin.note')),
                validator: (v) => (v ?? '').trim().isEmpty ? req : null,
              ),
              if (_noChange)
                Text(
                  context.t('admin.edit.no_change'),
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
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
          key: const Key('edit.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}

/// Return (11.7 item 6): kon item stock e ferot (default kono ta na: nosto/pocha jinish stock e na jay) ar note.
/// [refund] hole taka ferot er lekha. Bondho korle null.
Future<(List<int>, String)?> showReturnDialog(
  BuildContext context,
  Fulfillment f, {
  required bool refund,
}) => showDialog<(List<int>, String)>(
  context: context,
  builder: (_) => _ReturnDialog([
    for (final i in f.items)
      if (i.status == 'active') i,
  ], refund: refund),
);

class _ReturnDialog extends StatefulWidget {
  final List<AdminItem> items;
  final bool refund;
  const _ReturnDialog(this.items, {required this.refund});

  @override
  State<_ReturnDialog> createState() => _ReturnDialogState();
}

class _ReturnDialogState extends State<_ReturnDialog> {
  final _form = GlobalKey<FormState>();
  final _note = TextEditingController();
  final _restock = <int>{};

  @override
  void dispose() {
    _note.dispose();
    super.dispose();
  }

  void _submit() {
    if (_form.currentState!.validate()) {
      Navigator.of(context).pop((
        [
          for (final i in widget.items)
            if (_restock.contains(i.id)) i.id,
        ],
        _note.text.trim(),
      ));
    }
  }

  @override
  Widget build(BuildContext context) {
    final req = context.t('admin.required');
    return AlertDialog(
      title: Text(context.t('admin.fulfillment.return')),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(context.t('admin.return.hint')),
              if (widget.refund) Text(context.t('admin.return.refund_hint')),
              const SizedBox(height: 8),
              Text(context.t('admin.return.restock')),
              for (final i in widget.items)
                CheckboxListTile(
                  key: Key('return.${i.id}'),
                  contentPadding: EdgeInsets.zero,
                  controlAffinity: ListTileControlAffinity.leading,
                  title: Text(_itemName(context, i)),
                  subtitle: Text(context.count(i.quantity)),
                  value: _restock.contains(i.id),
                  onChanged: (v) => setState(
                    () => v! ? _restock.add(i.id) : _restock.remove(i.id),
                  ),
                ),
              TextFormField(
                key: const Key('return.note'),
                controller: _note,
                maxLength: 500,
                decoration: InputDecoration(labelText: context.t('admin.note')),
                validator: (v) => (v ?? '').trim().isEmpty ? req : null,
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
          key: const Key('return.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
