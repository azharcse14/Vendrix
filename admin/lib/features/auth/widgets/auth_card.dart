import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';

/// Login/setup er page er gora: majhe card, upore bhasha bodol, [error] er lekha.
/// [onSubmit] cholar shomoy (porer AuthState porjonto) button bondho.
class AuthCard extends StatefulWidget {
  final String title;
  final String? hint;
  final ApiError? error;
  final List<Widget> fields;
  final String submitLabel;
  final void Function() onSubmit;
  final List<Widget> actions;

  const AuthCard({
    super.key,
    required this.title,
    this.hint,
    this.error,
    required this.fields,
    required this.submitLabel,
    required this.onSubmit,
    this.actions = const [],
  });

  @override
  State<AuthCard> createState() => _AuthCardState();
}

class _AuthCardState extends State<AuthCard> {
  final _form = GlobalKey<FormState>();
  var _busy = false;

  void _submit() {
    if (_busy || !_form.currentState!.validate()) return;
    setState(() => _busy = true);
    widget.onSubmit();
  }

  @override
  Widget build(BuildContext context) {
    final text = Theme.of(context).textTheme;
    final err = widget.error;
    return BlocListener<AuthBloc, AuthState>(
      listener: (_, _) => setState(() => _busy = false),
      child: Scaffold(
        appBar: AppBar(
          actions: [
            TextButton(
              onPressed: context.read<LocaleCubit>().toggle,
              child: Text(context.t('admin.language')),
            ),
          ],
        ),
        body: Center(
          child: SingleChildScrollView(
            padding: const EdgeInsets.all(16),
            child: ConstrainedBox(
              constraints: const BoxConstraints(maxWidth: 400),
              child: Form(
                key: _form,
                child: AutofillGroup(
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.stretch,
                    children: [
                      Text(widget.title, style: text.headlineSmall),
                      if (widget.hint != null) ...[
                        const SizedBox(height: 8),
                        Text(widget.hint!),
                      ],
                      const SizedBox(height: 24),
                      for (final f in widget.fields) ...[
                        f,
                        const SizedBox(height: 16),
                      ],
                      if (err != null) ...[
                        Text(
                          context.errorText(err),
                          style: TextStyle(
                            color: Theme.of(context).colorScheme.error,
                          ),
                          semanticsLabel: context.errorText(err),
                        ),
                        const SizedBox(height: 16),
                      ],
                      FilledButton(
                        onPressed: _busy ? null : _submit,
                        child: Text(widget.submitLabel),
                      ),
                      ...widget.actions,
                    ],
                  ),
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}

/// Khali hole `admin.required`.
String? Function(String?) requiredField(BuildContext context) {
  final msg = context.t('admin.required');
  return (v) => v == null || v.trim().isEmpty ? msg : null;
}
