import 'package:flutter/material.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';

/// Batch form er uttor (khali field null).
typedef BatchInput = ({
  int quantity,
  int? costPrice,
  String? batchCode,
  String? harvestDate,
  String? expiryDate,
});

/// Komano form er uttor ([quantity] > 0).
typedef ReduceInput = ({int quantity, String reason, String note});

const _maxQty = 1000000; // inventory handler er maxQty

final _dateRe = RegExp(r'^\d{4}-\d{2}-\d{2}$');

/// `YYYY-MM-DD` ar asol din (2026-13-40 na).
bool validDate(String s) {
  if (!_dateRe.hasMatch(s)) return false;
  final d = DateTime.tryParse(s);
  return d != null && d.toIso8601String().startsWith(s);
}

String? _orNull(String s) => s.trim().isEmpty ? null : s.trim();

int? _qty(String? v) {
  final n = int.tryParse((v ?? '').trim());
  return n != null && n >= 1 && n <= _maxQty ? n : null;
}

/// Stock entry (`POST /batches`, 11.7 item 2); [showCost] false hole (`staff`, `vendor_staff`) kena dam field nai.
Future<BatchInput?> showBatchDialog(
  BuildContext context, {
  required bool showCost,
}) => showDialog<BatchInput>(
  context: context,
  builder: (_) => _BatchDialog(showCost),
);

class _BatchDialog extends StatefulWidget {
  final bool showCost;
  const _BatchDialog(this.showCost);

  @override
  State<_BatchDialog> createState() => _BatchDialogState();
}

class _BatchDialogState extends State<_BatchDialog> {
  final _form = GlobalKey<FormState>();
  final _qtyC = TextEditingController();
  final _cost = TextEditingController();
  final _code = TextEditingController();
  final _harvest = TextEditingController();
  final _expiry = TextEditingController();

  @override
  void dispose() {
    for (final c in [_qtyC, _cost, _code, _harvest, _expiry]) {
      c.dispose();
    }
    super.dispose();
  }

  void _submit() {
    if (!_form.currentState!.validate()) return;
    Navigator.of(context).pop((
      quantity: _qty(_qtyC.text)!,
      costPrice: _cost.text.trim().isEmpty ? null : parseTaka(_cost.text),
      batchCode: _orNull(_code.text),
      harvestDate: _orNull(_harvest.text),
      expiryDate: _orNull(_expiry.text),
    ));
  }

  @override
  Widget build(BuildContext context) {
    final badQty = context.t('admin.stock.quantity_invalid');
    final badTaka = context.t('admin.taka_invalid');
    final badDate = context.t('admin.date_invalid');
    String? date(String? v) =>
        (v ?? '').trim().isEmpty || validDate(v!.trim()) ? null : badDate;
    Widget field(
      String key,
      TextEditingController c,
      String label, {
      String? Function(String?)? validator,
      TextInputType? keyboard,
      String? hint,
    }) => TextFormField(
      key: Key('batch.$key'),
      controller: c,
      keyboardType: keyboard,
      decoration: InputDecoration(labelText: context.t(label), hintText: hint),
      validator: validator,
    );
    return AlertDialog(
      title: Text(context.t('admin.stock.add')),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              field(
                'quantity',
                _qtyC,
                'admin.stock.quantity',
                keyboard: TextInputType.number,
                validator: (v) => _qty(v) == null ? badQty : null,
              ),
              if (widget.showCost)
                field(
                  'cost',
                  _cost,
                  'admin.stock.cost_price',
                  keyboard: const TextInputType.numberWithOptions(
                    decimal: true,
                  ),
                  validator: (v) =>
                      (v ?? '').trim().isEmpty || parseTaka(v!) != null
                      ? null
                      : badTaka,
                ),
              field('code', _code, 'admin.stock.batch_code'),
              field(
                'harvest',
                _harvest,
                'admin.stock.harvest_date',
                hint: 'YYYY-MM-DD',
                validator: date,
              ),
              field(
                'expiry',
                _expiry,
                'admin.stock.expiry_date',
                hint: 'YYYY-MM-DD',
                validator: (v) {
                  final e = date(v);
                  final h = _harvest.text.trim();
                  if (e != null || (v ?? '').trim().isEmpty || !validDate(h)) {
                    return e;
                  }
                  return v!.trim().compareTo(h) < 0 ? badDate : null;
                },
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
          key: const Key('batch.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}

/// Komano (`POST /inventory/adjust`, shudhu komano, 11.7 item 2): koto, karon (`wastage`/`adjust`), note 1–500.
Future<ReduceInput?> showReduceDialog(BuildContext context) =>
    showDialog<ReduceInput>(context: context, builder: (_) => _ReduceDialog());

class _ReduceDialog extends StatefulWidget {
  @override
  State<_ReduceDialog> createState() => _ReduceDialogState();
}

class _ReduceDialogState extends State<_ReduceDialog> {
  final _form = GlobalKey<FormState>();
  final _qtyC = TextEditingController();
  final _note = TextEditingController();
  var _reason = 'wastage';

  @override
  void dispose() {
    _qtyC.dispose();
    _note.dispose();
    super.dispose();
  }

  void _submit() {
    if (!_form.currentState!.validate()) return;
    Navigator.of(context).pop((
      quantity: _qty(_qtyC.text)!,
      reason: _reason,
      note: _note.text.trim(),
    ));
  }

  @override
  Widget build(BuildContext context) {
    final badQty = context.t('admin.stock.quantity_invalid');
    final req = context.t('admin.required');
    return AlertDialog(
      title: Text(context.t('admin.stock.reduce')),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                key: const Key('adjust.quantity'),
                controller: _qtyC,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(
                  labelText: context.t('admin.stock.reduce_quantity'),
                ),
                validator: (v) => _qty(v) == null ? badQty : null,
              ),
              DropdownButtonFormField<String>(
                key: const Key('adjust.reason'),
                isExpanded: true,
                initialValue: _reason,
                decoration: InputDecoration(
                  labelText: context.t('admin.stock.reason'),
                ),
                items: [
                  for (final r in ['wastage', 'adjust'])
                    DropdownMenuItem(
                      value: r,
                      child: Text(context.t('admin.reason.$r')),
                    ),
                ],
                onChanged: (v) => _reason = v!,
              ),
              TextFormField(
                key: const Key('adjust.note'),
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
          key: const Key('adjust.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
