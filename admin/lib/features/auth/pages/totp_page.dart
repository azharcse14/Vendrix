import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../widgets/auth_card.dart';

/// `/login/totp`: authenticator er code ba recovery code (8.3 step 2-3).
class TotpPage extends StatefulWidget {
  const TotpPage({super.key});

  @override
  State<TotpPage> createState() => _TotpPageState();
}

class _TotpPageState extends State<TotpPage> {
  final _code = TextEditingController();
  var _recovery = false;

  @override
  void dispose() {
    _code.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = context.watch<AuthBloc>().state;
    return AuthCard(
      title: context.t('admin.totp.title'),
      hint: context.t(
        _recovery ? 'admin.totp.recovery_hint' : 'admin.totp.hint',
      ),
      error: s is TotpRequired ? s.error : null,
      submitLabel: context.t('admin.continue'),
      onSubmit: () => context.read<AuthBloc>().add(
        MfaSubmitted(_code.text.trim(), recovery: _recovery),
      ),
      fields: [
        TextFormField(
          controller: _code,
          decoration: InputDecoration(labelText: context.t('admin.totp.code')),
          keyboardType: _recovery ? TextInputType.text : TextInputType.number,
          autofillHints: const [AutofillHints.oneTimeCode],
          validator: requiredField(context),
        ),
      ],
      actions: [
        TextButton(
          onPressed: () => setState(() {
            _recovery = !_recovery;
            _code.clear();
          }),
          child: Text(
            context.t(
              _recovery ? 'admin.totp.use_totp' : 'admin.totp.use_recovery',
            ),
          ),
        ),
        TextButton(
          onPressed: () => context.read<AuthBloc>().add(LogoutRequested()),
          child: Text(context.t('admin.totp.back')),
        ),
      ],
    );
  }
}
