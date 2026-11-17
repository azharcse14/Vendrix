import 'package:flutter/material.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';

final _code = RegExp(r'^[A-Z0-9_-]{1,32}$');

// ponytail: shop er din Asia/Dhaka (+06:00, shared formatDate er moto); onno timezone MT e.
const _offset = Duration(hours: 6);

String _day(DateTime? t) =>
    t == null ? '' : t.toUtc().add(_offset).toIso8601String().substring(0, 10);

/// Coupon form (11.7 item 13 v): puro object (create ar PATCH e ekoi; lock field ekoi value e server e chole) ba
/// null. `code` trim + upper; percent 1–100, fixed taka → poisha; tarikh shop er din: shuru 00:00, shesh 23:59:59.
/// `used_count > 0` hole code/kind/value bodlano jay na.
Future<Map<String, Object?>?> showCouponDialog(
  BuildContext context,
  Coupon? cp,
) => showDialog<Map<String, Object?>>(
  context: context,
  builder: (_) => _CouponDialog(cp),
);

class _CouponDialog extends StatefulWidget {
  final Coupon? cp;
  const _CouponDialog(this.cp);

  @override
  State<_CouponDialog> createState() => _CouponDialogState();
}

class _CouponDialogState extends State<_CouponDialog> {
  final _form = GlobalKey<FormState>();
  late final cp = widget.cp;
  late final _locked = (cp?.usedCount ?? 0) > 0;
  late var _kind = cp?.kind ?? 'percent';
  late final _codeC = TextEditingController(text: cp?.code);
  late final _value = TextEditingController(
    text: cp == null
        ? ''
        : cp!.kind == 'percent'
        ? '${cp!.value}'
        : takaInput(cp!.value),
  );
  late final _min = TextEditingController(text: takaInput(cp?.minOrder ?? 0));
  late final _max = TextEditingController(text: takaInput(cp?.maxDiscount));
  late final _limit = TextEditingController(
    text: cp?.usageLimit?.toString() ?? '',
  );
  late final _perPhone = TextEditingController(
    text: '${cp?.perPhoneLimit ?? 1}',
  );
  late final _starts = TextEditingController(text: _day(cp?.startsAt));
  late final _expires = TextEditingController(text: _day(cp?.expiresAt));
  late var _active = cp?.isActive ?? true;

  @override
  void dispose() {
    for (final c in [
      _codeC,
      _value,
      _min,
      _max,
      _limit,
      _perPhone,
      _starts,
      _expires,
    ]) {
      c.dispose();
    }
    super.dispose();
  }

  int? _valueOf(String s) {
    if (_kind == 'fixed') {
      final p = parseTaka(s);
      return p != null && p > 0 ? p : null;
    }
    final n = int.tryParse(s.trim());
    return n != null && n >= 1 && n <= 100 ? n : null;
  }

  String? _at(TextEditingController c, String time) =>
      c.text.trim().isEmpty ? null : '${c.text.trim()}T$time+06:00';

  void _submit() {
    if (!_form.currentState!.validate()) return;
    final limit = _limit.text.trim();
    Navigator.of(context).pop({
      'code': _codeC.text.trim().toUpperCase(),
      'kind': _kind,
      'value': _valueOf(_value.text),
      'min_order': parseTaka(_min.text),
      'max_discount': _max.text.trim().isEmpty ? null : parseTaka(_max.text),
      'usage_limit': limit.isEmpty ? null : int.parse(limit),
      'per_phone_limit': int.parse(_perPhone.text.trim()),
      'starts_at': _at(_starts, '00:00:00'),
      'expires_at': _at(_expires, '23:59:59'),
      'is_active': _active,
    });
  }

  @override
  Widget build(BuildContext context) {
    final badCode = context.t('admin.coupon.code_invalid');
    final badValue = context.t(
      _kind == 'percent'
          ? 'admin.coupon.percent_invalid'
          : 'admin.taka_invalid',
    );
    final badTaka = context.t('admin.taka_invalid');
    final badNumber = context.t('admin.number_invalid');
    final belowUsed = context.t('admin.coupon.below_used');
    final badDate = context.t('admin.date_invalid');
    final badRange = context.t('admin.coupon.dates_invalid');
    final used = cp?.usedCount ?? 0;
    String? date(String? v) {
      final t = (v ?? '').trim();
      return t.isEmpty || validDate(t) ? null : badDate;
    }

    TextFormField field(
      String key,
      TextEditingController c,
      String label,
      String? Function(String?) validator, {
      bool enabled = true,
      String? helper,
    }) => TextFormField(
      key: Key('coupon.$key'),
      controller: c,
      enabled: enabled,
      decoration: InputDecoration(
        labelText: context.t(label),
        helperText: helper,
      ),
      validator: validator,
    );

    return AlertDialog(
      title: Text(
        context.t(cp == null ? 'admin.coupon.new' : 'admin.coupon.edit'),
      ),
      content: Form(
        key: _form,
        child: SingleChildScrollView(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              if (_locked) Text(context.t('admin.coupon.locked')),
              field(
                'code',
                _codeC,
                'admin.coupon.code',
                (v) => _code.hasMatch((v ?? '').trim().toUpperCase())
                    ? null
                    : badCode,
                enabled: !_locked,
              ),
              DropdownButtonFormField<String>(
                key: const Key('coupon.kind'),
                isExpanded: true,
                initialValue: _kind,
                decoration: InputDecoration(
                  labelText: context.t('admin.coupon.kind'),
                ),
                items: [
                  for (final k in ['percent', 'fixed'])
                    DropdownMenuItem(
                      key: Key('kind.$k'),
                      value: k,
                      child: Text(context.t('admin.coupon.kind.$k')),
                    ),
                ],
                onChanged: _locked ? null : (v) => setState(() => _kind = v!),
              ),
              field(
                'value',
                _value,
                _kind == 'percent'
                    ? 'admin.coupon.value_percent'
                    : 'admin.coupon.value_fixed',
                (v) => _valueOf(v ?? '') == null ? badValue : null,
                enabled: !_locked,
              ),
              field(
                'min_order',
                _min,
                'admin.zone.min_order',
                (v) => parseTaka(v ?? '') == null ? badTaka : null,
              ),
              field('max_discount', _max, 'admin.coupon.max_discount', (v) {
                final t = (v ?? '').trim();
                final p = parseTaka(t);
                return t.isEmpty || (p != null && p > 0) ? null : badTaka;
              }),
              field(
                'usage_limit',
                _limit,
                'admin.coupon.usage_limit',
                (v) {
                  final t = (v ?? '').trim();
                  if (t.isEmpty) return null;
                  final n = int.tryParse(t);
                  if (n == null || n < 1 || n > 2147483647) return badNumber;
                  return n < used ? belowUsed : null;
                },
                helper: context.t('admin.coupon.used', {
                  'n': context.count(used),
                }),
              ),
              field('per_phone', _perPhone, 'admin.coupon.per_phone', (v) {
                final n = int.tryParse((v ?? '').trim());
                return n != null && n >= 1 && n <= 2147483647
                    ? null
                    : badNumber;
              }),
              field('starts', _starts, 'admin.coupon.starts', date),
              field('expires', _expires, 'admin.coupon.expires', (v) {
                final bad = date(v);
                if (bad != null) return bad;
                final s = _starts.text.trim(), e = (v ?? '').trim();
                return s.isNotEmpty &&
                        e.isNotEmpty &&
                        validDate(s) &&
                        s.compareTo(e) > 0
                    ? badRange
                    : null;
              }),
              SwitchListTile(
                key: const Key('coupon.active'),
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
          key: const Key('coupon.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
