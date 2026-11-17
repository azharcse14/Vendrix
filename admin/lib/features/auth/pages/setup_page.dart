import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../widgets/auth_card.dart';

/// `/login/setup`: password → TOTP → recovery code (8.3 step 4).
class SetupPage extends StatefulWidget {
  const SetupPage({super.key});

  @override
  State<SetupPage> createState() => _SetupPageState();
}

class _SetupPageState extends State<SetupPage> {
  final _old = TextEditingController();
  final _new = TextEditingController();
  final _code = TextEditingController();

  @override
  void dispose() {
    for (final c in [_old, _new, _code]) {
      c.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = context.watch<AuthBloc>().state;
    // redirect er age ek frame
    if (s is! SetupRequired) return const SizedBox.shrink();
    final bloc = context.read<AuthBloc>();
    final req = requiredField(context);
    switch (s.step) {
      case SetupStep.password:
        // field er bhul nijer lekha te, baki error.<code>
        final f = s.error?.fields ?? const {};
        return AuthCard(
          key: const ValueKey(SetupStep.password),
          title: context.t('admin.setup.password_title'),
          hint: context.t('admin.setup.password_hint'),
          error: f.isEmpty ? s.error : null,
          submitLabel: context.t('admin.continue'),
          onSubmit: () => bloc.add(PasswordSubmitted(_old.text, _new.text)),
          fields: [
            TextFormField(
              controller: _old,
              obscureText: true,
              autofillHints: const [AutofillHints.password],
              decoration: InputDecoration(
                labelText: context.t('admin.setup.old'),
                errorText: f.containsKey('old')
                    ? context.t('admin.setup.old_wrong')
                    : null,
              ),
              validator: req,
            ),
            TextFormField(
              controller: _new,
              obscureText: true,
              autofillHints: const [AutofillHints.newPassword],
              decoration: InputDecoration(
                labelText: context.t('admin.setup.new'),
                errorText: f.containsKey('new')
                    ? context.t('admin.setup.new_invalid')
                    : null,
              ),
              validator: req,
            ),
          ],
        );
      case SetupStep.totp:
        return AuthCard(
          key: const ValueKey(SetupStep.totp),
          title: context.t('admin.setup.totp_title'),
          hint: context.t('admin.setup.totp_hint'),
          error: s.error,
          submitLabel: context.t('admin.continue'),
          onSubmit: () => bloc.add(TotpEnableSubmitted(_code.text.trim())),
          fields: [
            _Copyable(
              label: context.t('admin.setup.secret'),
              text: s.secret ?? '',
            ),
            TextFormField(
              controller: _code,
              keyboardType: TextInputType.number,
              autofillHints: const [AutofillHints.oneTimeCode],
              decoration: InputDecoration(
                labelText: context.t('admin.totp.code'),
              ),
              validator: req,
            ),
          ],
        );
      case SetupStep.recoveryCodes:
        return AuthCard(
          key: const ValueKey(SetupStep.recoveryCodes),
          title: context.t('admin.setup.recovery_title'),
          hint: context.t('admin.setup.recovery_hint'),
          submitLabel: context.t('admin.setup.recovery_done'),
          onSubmit: () => bloc.add(RecoveryCodesSaved()),
          fields: [
            _Copyable(
              label: context.t('admin.setup.recovery_title'),
              text: s.recoveryCodes.join('\n'),
            ),
          ],
        );
    }
  }
}

/// Select kora jay emon lekha + copy button.
class _Copyable extends StatelessWidget {
  final String label, text;
  const _Copyable({required this.label, required this.text});

  @override
  Widget build(BuildContext context) => InputDecorator(
    decoration: InputDecoration(
      labelText: label,
      suffixIcon: IconButton(
        icon: const Icon(Icons.copy),
        tooltip: label,
        onPressed: () => Clipboard.setData(ClipboardData(text: text)),
      ),
    ),
    child: SelectableText(
      text,
      style: const TextStyle(fontFamily: 'monospace', letterSpacing: 1),
    ),
  );
}
