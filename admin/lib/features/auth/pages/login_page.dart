import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../widgets/auth_card.dart';

// Single store e build e shop er slug (16.1 na, M7b step 1 decision); khali hole staff nije dey.
const _shopSlug = String.fromEnvironment('SHOP_SLUG');

/// `/login`: shop, email, password (8.3 step 1).
class LoginPage extends StatefulWidget {
  const LoginPage({super.key});

  @override
  State<LoginPage> createState() => _LoginPageState();
}

class _LoginPageState extends State<LoginPage> {
  final _shop = TextEditingController(text: _shopSlug);
  final _email = TextEditingController();
  final _password = TextEditingController();

  @override
  void dispose() {
    for (final c in [_shop, _email, _password]) {
      c.dispose();
    }
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final s = context.watch<AuthBloc>().state;
    final req = requiredField(context);
    return AuthCard(
      title: context.t('admin.login.title'),
      error: s is Unauthenticated ? s.error : null,
      submitLabel: context.t('admin.login.submit'),
      onSubmit: () => context.read<AuthBloc>().add(
        LoginSubmitted(_shop.text.trim(), _email.text.trim(), _password.text),
      ),
      fields: [
        TextFormField(
          controller: _shop,
          decoration: InputDecoration(labelText: context.t('admin.login.shop')),
          validator: req,
        ),
        TextFormField(
          controller: _email,
          decoration: InputDecoration(
            labelText: context.t('admin.login.email'),
          ),
          keyboardType: TextInputType.emailAddress,
          autofillHints: const [AutofillHints.username, AutofillHints.email],
          validator: req,
        ),
        TextFormField(
          controller: _password,
          decoration: InputDecoration(
            labelText: context.t('admin.login.password'),
          ),
          obscureText: true,
          autofillHints: const [AutofillHints.password],
          validator: req,
        ),
      ],
    );
  }
}
