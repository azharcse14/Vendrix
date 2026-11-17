import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/staff_cubit.dart';

/// Shop staff er role (vendor role MV e).
const staffRoles = ['owner', 'admin', 'staff'];

/// `/staff`: list, notun, role, active, reset; nijer row e kaj nai (server o `403`).
class StaffPage extends StatelessWidget {
  const StaffPage({super.key});

  Future<void> _create(BuildContext context) async {
    final c = context.read<StaffCubit>();
    final r = await showDialog<(String, String, String)>(
      context: context,
      builder: (_) => const _CreateDialog(),
    );
    if (r == null) return;
    final temp = await c.create(r.$1, r.$2, r.$3);
    if (temp != null && context.mounted) await _showTemp(context, temp);
  }

  Future<void> _reset(BuildContext context, int id) async {
    final c = context.read<StaffCubit>();
    if (!await confirmDialog(context, 'admin.staff.reset_confirm')) return;
    final temp = await c.reset(id);
    if (temp != null && context.mounted) await _showTemp(context, temp);
  }

  @override
  Widget build(BuildContext context) {
    final c = context.read<StaffCubit>();
    final auth = context.watch<AuthBloc>().state;
    final me = auth is Authenticated ? auth.staff.email : '';
    return Scaffold(
      body: switch (context.watch<StaffCubit>().state) {
        StaffLoading() => const Center(child: CircularProgressIndicator()),
        StaffFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        final StaffLoaded s => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (s.error != null) ErrorLine(context.errorText(s.error!)),
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: FilledButton.icon(
                key: const Key('staff.new'),
                onPressed: s.busy ? null : () => _create(context),
                icon: const Icon(Icons.person_add_outlined),
                label: Text(context.t('admin.staff.new')),
              ),
            ),
            for (final st in s.staff) _row(context, st, s.busy, st.email == me),
          ],
        ),
      },
    );
  }

  Widget _row(BuildContext context, Staff st, bool busy, bool self) {
    final c = context.read<StaffCubit>();
    return Card(
      key: Key('staff.${st.id}'),
      margin: const EdgeInsets.only(top: 8),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(st.name, style: Theme.of(context).textTheme.titleMedium),
            Text(st.email),
            Text(
              [
                if (!st.isActive) context.t('admin.inactive'),
                if (st.mustChangePassword)
                  context.t('admin.staff.setup_pending'),
                if (st.lockedUntil != null) context.t('admin.staff.locked'),
                if (st.lastLoginAt != null)
                  context.t('admin.staff.last_login', {
                    'date': context.date(st.lastLoginAt!),
                  }),
              ].join(' · '),
            ),
            const SizedBox(height: 8),
            if (self)
              Text(context.t('admin.role.${st.role}'))
            else
              Wrap(
                spacing: 8,
                runSpacing: 8,
                crossAxisAlignment: WrapCrossAlignment.center,
                children: [
                  DropdownButton<String>(
                    key: Key('staff.${st.id}.role'),
                    value: staffRoles.contains(st.role) ? st.role : null,
                    items: [
                      for (final r in staffRoles)
                        DropdownMenuItem(
                          key: Key('role.$r'),
                          value: r,
                          child: Text(context.t('admin.role.$r')),
                        ),
                    ],
                    onChanged: busy
                        ? null
                        : (r) => r == st.role ? null : c.update(st.id, role: r),
                  ),
                  Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Switch(
                        key: Key('staff.${st.id}.active'),
                        value: st.isActive,
                        onChanged: busy
                            ? null
                            : (v) => c.update(st.id, isActive: v),
                      ),
                      Text(context.t('admin.active')),
                    ],
                  ),
                  OutlinedButton(
                    key: Key('staff.${st.id}.reset'),
                    onPressed: busy ? null : () => _reset(context, st.id),
                    child: Text(context.t('admin.staff.reset')),
                  ),
                ],
              ),
          ],
        ),
      ),
    );
  }
}

/// Temporary password ekbar: copy kore staff ke dao; bondho korle ar dekha jay na.
Future<void> _showTemp(BuildContext context, String temp) => showDialog<void>(
  context: context,
  barrierDismissible: false,
  builder: (context) => AlertDialog(
    title: Text(context.t('admin.staff.temp_title')),
    content: Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Text(context.t('admin.staff.temp_hint')),
        const SizedBox(height: 12),
        SelectableText(temp, style: Theme.of(context).textTheme.titleMedium),
      ],
    ),
    actions: [
      TextButton(
        key: const Key('temp.copy'),
        onPressed: () => Clipboard.setData(ClipboardData(text: temp)),
        child: Text(context.t('admin.copy')),
      ),
      FilledButton(
        key: const Key('temp.close'),
        onPressed: () => Navigator.of(context).pop(),
        child: Text(context.t('admin.close')),
      ),
    ],
  ),
);

class _CreateDialog extends StatefulWidget {
  const _CreateDialog();

  @override
  State<_CreateDialog> createState() => _CreateDialogState();
}

class _CreateDialogState extends State<_CreateDialog> {
  final _form = GlobalKey<FormState>();
  final _email = TextEditingController();
  final _name = TextEditingController();
  var _role = 'staff';

  @override
  void dispose() {
    _email.dispose();
    _name.dispose();
    super.dispose();
  }

  void _submit() {
    if (_form.currentState!.validate()) {
      Navigator.of(context).pop((_email.text.trim(), _name.text.trim(), _role));
    }
  }

  @override
  Widget build(BuildContext context) {
    final req = context.t('admin.required');
    final badEmail = context.t('admin.email_invalid');
    return AlertDialog(
      title: Text(context.t('admin.staff.new')),
      content: Form(
        key: _form,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextFormField(
              key: const Key('staff.email'),
              controller: _email,
              keyboardType: TextInputType.emailAddress,
              maxLength: 254,
              decoration: InputDecoration(
                labelText: context.t('admin.login.email'),
              ),
              // pura check server e (net/mail); ekhane shudhu spashto bhul
              validator: (v) {
                final t = (v ?? '').trim();
                return t.contains('@') && !t.contains(' ') ? null : badEmail;
              },
            ),
            TextFormField(
              key: const Key('staff.name'),
              controller: _name,
              maxLength: 100,
              decoration: InputDecoration(
                labelText: context.t('admin.staff.name'),
              ),
              validator: (v) => (v ?? '').trim().isEmpty ? req : null,
            ),
            DropdownButtonFormField<String>(
              key: const Key('staff.role'),
              isExpanded: true,
              initialValue: _role,
              decoration: InputDecoration(
                labelText: context.t('admin.staff.role'),
              ),
              items: [
                for (final r in staffRoles)
                  DropdownMenuItem(
                    value: r,
                    child: Text(context.t('admin.role.$r')),
                  ),
              ],
              onChanged: (v) => _role = v!,
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(context.t('admin.close')),
        ),
        FilledButton(
          key: const Key('staff.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
