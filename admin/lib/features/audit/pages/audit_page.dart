import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/audit_cubit.dart';

const _json = JsonEncoder.withIndent('  ');

/// `/audit`: filter ar list; row khulle before/after JSON (server e phone/password thake na, 12.2).
class AuditPage extends StatelessWidget {
  const AuditPage({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.read<AuditCubit>();
    final s = context.watch<AuditCubit>().state;
    return Scaffold(
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          _Filter(c),
          ...switch (s) {
            AuditLoading() => [
              const Center(child: CircularProgressIndicator()),
            ],
            AuditFailed(:final error) => [ErrorRetry(error, onRetry: c.load)],
            AuditLoaded(:final page) => [
              if (page.items.isEmpty) Text(context.t('admin.empty')),
              for (final e in page.items)
                Card(
                  key: Key('audit.${e.id}'),
                  margin: const EdgeInsets.only(top: 8),
                  child: ExpansionTile(
                    title: Text(e.action),
                    subtitle: Text(
                      [
                        e.staffName ?? context.t('admin.audit.system'),
                        '${e.entityType} ${e.entityId ?? ''}'.trim(),
                        context.date(e.createdAt),
                        ?e.ip,
                      ].join(' · '),
                    ),
                    expandedCrossAxisAlignment: CrossAxisAlignment.start,
                    childrenPadding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                    children: [
                      for (final (k, v) in [
                        ('admin.audit.before', e.before),
                        ('admin.audit.after', e.after),
                      ])
                        if (v != null) ...[
                          Text(
                            context.t(k),
                            style: Theme.of(context).textTheme.titleSmall,
                          ),
                          SelectableText(_json.convert(v)),
                        ],
                    ],
                  ),
                ),
              PageBar(
                prefix: 'audit',
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

class _Filter extends StatefulWidget {
  final AuditCubit c;
  const _Filter(this.c);

  @override
  State<_Filter> createState() => _FilterState();
}

class _FilterState extends State<_Filter> {
  final _form = GlobalKey<FormState>();
  final _action = TextEditingController();
  final _staff = TextEditingController();
  final _from = TextEditingController();
  final _to = TextEditingController();

  @override
  void dispose() {
    for (final c in [_action, _staff, _from, _to]) {
      c.dispose();
    }
    super.dispose();
  }

  String? _or(TextEditingController c) =>
      c.text.trim().isEmpty ? null : c.text.trim();

  void _apply() {
    if (!_form.currentState!.validate()) return;
    widget.c.apply((
      action: _or(_action),
      staffId: int.tryParse(_staff.text.trim()),
      from: _or(_from),
      to: _or(_to),
    ));
  }

  @override
  Widget build(BuildContext context) {
    final badDate = context.t('admin.date_invalid');
    final badNumber = context.t('admin.number_invalid');
    String? date(String? v) {
      final t = (v ?? '').trim();
      return t.isEmpty || validDate(t) ? null : badDate;
    }

    Widget box(Widget w) => SizedBox(width: 200, child: w);
    return Form(
      key: _form,
      child: Wrap(
        spacing: 8,
        runSpacing: 8,
        crossAxisAlignment: WrapCrossAlignment.end,
        children: [
          box(
            TextFormField(
              key: const Key('audit.action'),
              controller: _action,
              maxLength: 64,
              decoration: InputDecoration(
                labelText: context.t('admin.audit.action'),
                counterText: '',
              ),
            ),
          ),
          box(
            TextFormField(
              key: const Key('audit.staff'),
              controller: _staff,
              keyboardType: TextInputType.number,
              decoration: InputDecoration(
                labelText: context.t('admin.audit.staff_id'),
              ),
              validator: (v) {
                final t = (v ?? '').trim();
                final n = int.tryParse(t);
                return t.isEmpty || (n != null && n > 0) ? null : badNumber;
              },
            ),
          ),
          box(
            TextFormField(
              key: const Key('audit.from'),
              controller: _from,
              decoration: InputDecoration(
                labelText: context.t('admin.audit.from'),
                hintText: context.t('admin.date_hint'),
              ),
              validator: date,
            ),
          ),
          box(
            TextFormField(
              key: const Key('audit.to'),
              controller: _to,
              decoration: InputDecoration(
                labelText: context.t('admin.audit.to'),
                hintText: context.t('admin.date_hint'),
              ),
              validator: date,
            ),
          ),
          FilledButton(
            key: const Key('audit.apply'),
            onPressed: _apply,
            child: Text(context.t('admin.audit.apply')),
          ),
        ],
      ),
    );
  }
}
