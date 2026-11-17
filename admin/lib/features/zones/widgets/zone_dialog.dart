import 'package:flutter/material.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';

/// Zone form (11.7 item 13 vi): body (puro object, create ar PATCH e ekoi) ba null. `name.bn` lagbe, district/area
/// trim max 100 (area khali = puro district), taka form e taka, body te poisha.
Future<Map<String, Object?>?> showZoneDialog(BuildContext context, Zone? z) =>
    showDialog<Map<String, Object?>>(
      context: context,
      builder: (_) => _ZoneDialog(z),
    );

class _ZoneDialog extends StatefulWidget {
  final Zone? z;
  const _ZoneDialog(this.z);

  @override
  State<_ZoneDialog> createState() => _ZoneDialogState();
}

class _ZoneDialogState extends State<_ZoneDialog> {
  final _form = GlobalKey<FormState>();
  late final _bn = TextEditingController(text: widget.z?.name.values['bn']);
  late final _en = TextEditingController(text: widget.z?.name.values['en']);
  late final _district = TextEditingController(text: widget.z?.district);
  late final _area = TextEditingController(text: widget.z?.area);
  late final _charge = TextEditingController(text: takaInput(widget.z?.charge));
  late final _min = TextEditingController(
    text: takaInput(widget.z?.minOrder ?? 0),
  );
  late final _free = TextEditingController(
    text: takaInput(widget.z?.freeDeliveryOver),
  );
  late var _sameCity =
      widget.z?.sameCity ?? false; // bhul true e 5 diner protishruti (8.15)
  late var _active = widget.z?.isActive ?? true;

  @override
  void dispose() {
    for (final c in [_bn, _en, _district, _area, _charge, _min, _free]) {
      c.dispose();
    }
    super.dispose();
  }

  void _submit() {
    if (!_form.currentState!.validate()) return;
    final area = _area.text.trim();
    Navigator.of(context).pop({
      'name': {'bn': _bn.text.trim(), 'en': _en.text.trim()},
      'district': _district.text.trim(),
      'area': area.isEmpty ? null : area,
      'charge': parseTaka(_charge.text),
      'min_order': parseTaka(_min.text),
      'free_delivery_over': _free.text.trim().isEmpty
          ? null
          : parseTaka(_free.text),
      'same_city': _sameCity,
      'is_active': _active,
    });
  }

  @override
  Widget build(BuildContext context) {
    final req = context.t('admin.required');
    final tooLong = context.t('admin.zone.place_invalid');
    final badTaka = context.t('admin.taka_invalid');
    String? place(String? v, {bool required = true}) {
      final t = (v ?? '').trim();
      if (t.isEmpty) return required ? req : null;
      return t.runes.length > 100 ? tooLong : null;
    }

    String? taka(String? v, {bool required = true, bool positive = false}) {
      final t = (v ?? '').trim();
      if (t.isEmpty && !required) return null;
      final p = parseTaka(t);
      return p == null || (positive && p == 0) ? badTaka : null;
    }

    return AlertDialog(
      title: Text(
        context.t(widget.z == null ? 'admin.zone.new' : 'admin.zone.edit'),
      ),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              TextFormField(
                key: const Key('zone.name_bn'),
                controller: _bn,
                decoration: InputDecoration(
                  labelText: context.t('admin.product.name_bn'),
                ),
                validator: (v) => (v ?? '').trim().isEmpty ? req : null,
              ),
              TextFormField(
                key: const Key('zone.name_en'),
                controller: _en,
                decoration: InputDecoration(
                  labelText: context.t('admin.product.name_en'),
                ),
              ),
              TextFormField(
                key: const Key('zone.district'),
                controller: _district,
                decoration: InputDecoration(
                  labelText: context.t('admin.zone.district'),
                ),
                validator: place,
              ),
              TextFormField(
                key: const Key('zone.area'),
                controller: _area,
                decoration: InputDecoration(
                  labelText: context.t('admin.zone.area'),
                  helperText: context.t('admin.zone.area_hint'),
                ),
                validator: (v) => place(v, required: false),
              ),
              TextFormField(
                key: const Key('zone.charge'),
                controller: _charge,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(
                  labelText: context.t('admin.zone.charge'),
                ),
                validator: taka,
              ),
              TextFormField(
                key: const Key('zone.min_order'),
                controller: _min,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(
                  labelText: context.t('admin.zone.min_order'),
                ),
                validator: taka,
              ),
              TextFormField(
                key: const Key('zone.free_over'),
                controller: _free,
                keyboardType: TextInputType.number,
                decoration: InputDecoration(
                  labelText: context.t('admin.zone.free_over'),
                ),
                validator: (v) => taka(v, required: false, positive: true),
              ),
              SwitchListTile(
                key: const Key('zone.same_city'),
                contentPadding: EdgeInsets.zero,
                title: Text(context.t('admin.zone.same_city')),
                subtitle: Text(context.t('admin.zone.same_city_hint')),
                value: _sameCity,
                onChanged: (v) => setState(() => _sameCity = v),
              ),
              SwitchListTile(
                key: const Key('zone.active'),
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
          key: const Key('zone.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
