import 'package:http/http.dart' as http;
import 'package:jaspr/jaspr.dart';
import 'package:shared/api/api_client.dart';
import 'package:universal_web/js_interop.dart';
import 'package:universal_web/web.dart' as web;

// Turnstile er token ekbar e shesh, tai protiti send er por notun widget token.
@JS('turnstile.reset')
external void _turnstileReset();

/// `/login` er kaj (10.11 item 1): `#login-phone` submit → BFF `/_/auth/otp/send` (JSON, Turnstile token shoho) →
/// `#login-code` → `/_/auth/otp/verify` → [next]. Phone shudhu memory te (cookie ba URL na). Lekha server e render
/// (`[data-error]`), tai client er data shudhu [next] (10.2: admin er lekha HTML comment e na).
@client
class LoginForm extends StatefulComponent {
  /// Login er por kothay (server e `safeNext` kora path).
  final String next;

  /// LoginForm.
  const LoginForm({required this.next, super.key});

  @override
  State<LoginForm> createState() => _LoginFormState();
}

class _LoginFormState extends State<LoginForm> {
  String _phone = '';
  bool _busy = false;
  late final ApiClient _bff = ApiClient(Uri.base, http.Client());

  @override
  void initState() {
    super.initState();
    if (!kIsWeb) return;
    final phoneForm = web.document.getElementById('login-phone') as web.HTMLFormElement?;
    final codeForm = web.document.getElementById('login-code') as web.HTMLFormElement?;
    if (phoneForm == null || codeForm == null) return;
    web.EventStreamProviders.submitEvent.forTarget(phoneForm).listen((e) {
      e.preventDefault();
      _send(phoneForm, codeForm);
    });
    web.EventStreamProviders.submitEvent.forTarget(codeForm).listen((e) {
      e.preventDefault();
      _verify(phoneForm, codeForm);
    });
    final back = codeForm.querySelector('[data-login-back]');
    if (back != null) {
      web.EventStreamProviders.clickEvent.forTarget(back).listen((_) => _show(phoneForm, codeForm, code: false));
    }
  }

  Future<void> _send(web.HTMLFormElement phoneForm, web.HTMLFormElement codeForm) async {
    if (_busy) return;
    _busy = true;
    _error(null);
    final phone = _value(phoneForm, 'phone');
    try {
      await _bff.send(
        'POST',
        '/_/auth/otp/send',
        body: {'phone': phone, 'purpose': 'login', 'turnstile_token': _value(phoneForm, 'turnstile_token')},
      );
      _phone = phone;
      _show(phoneForm, codeForm, code: true);
    } on ApiError catch (e) {
      _error(e.code);
    } finally {
      _busy = false;
      _resetTurnstile();
    }
  }

  Future<void> _verify(web.HTMLFormElement phoneForm, web.HTMLFormElement codeForm) async {
    if (_busy) return;
    _busy = true;
    _error(null);
    try {
      final res = await _bff.send(
        'POST',
        '/_/auth/otp/verify',
        body: {'phone': _phone, 'code': _value(codeForm, 'code')},
      );
      // Cookie BFF set korechhe; _busy thake jate abar submit na hoy. Merge e item bad gele age line ta (10.12 item 2).
      if (_notices(res)) await Future<void>.delayed(const Duration(seconds: 4));
      web.window.location.assign(component.next);
      return;
    } on ApiError catch (e) {
      _error(e.code);
      // Ei code ar chole na, notun code lagbe.
      if (e.code == 'otp_too_many_attempts') _show(phoneForm, codeForm, code: false);
    }
    _busy = false;
  }

  // `cart_notices` er lekha `#login-notice` e; kichu dekhale true.
  bool _notices(Object? res) {
    final box = web.document.getElementById('login-notice');
    if (box == null || res is! Map || res['cart_notices'] is! List) return false;
    final lines = [
      for (final n in res['cart_notices'] as List)
        if (n case {'code': final String code, 'name': final String name} when RegExp(r'^[a-z_]+$').hasMatch(code))
          if (web.document.querySelector('[data-notice="$code"]')?.textContent case final t?)
            t.replaceAll('{name}', name),
    ];
    if (lines.isEmpty) return false;
    box.textContent = lines.join('\n');
    box.removeAttribute('hidden');
    return true;
  }

  void _show(web.HTMLFormElement phoneForm, web.HTMLFormElement codeForm, {required bool code}) {
    phoneForm.toggleAttribute('hidden', code);
    codeForm.toggleAttribute('hidden', !code);
    final input = (code ? codeForm : phoneForm).querySelector('input:not([type="hidden"])') as web.HTMLInputElement?;
    if (code) input?.value = '';
    input?.focus();
  }

  String _value(web.HTMLFormElement form, String name) =>
      (form.querySelector('[name="$name"]') as web.HTMLInputElement?)?.value ?? '';

  // Server er render kora lekha; ochena code e `internal`.
  void _error(String? code) {
    final box = web.document.getElementById('login-error');
    if (box == null) return;
    // Code selector e jay, tai shudhu Go er dhoron (`[a-z_]`).
    final known = code != null && RegExp(r'^[a-z_]+$').hasMatch(code) ? code : 'internal';
    final text = code == null
        ? null
        : (web.document.querySelector('[data-error="$known"]') ?? web.document.querySelector('[data-error="internal"]'))
              ?.textContent;
    box.textContent = text ?? '';
    box.toggleAttribute('hidden', text == null);
  }

  void _resetTurnstile() {
    try {
      _turnstileReset();
    } catch (_) {
      // Script ashe ni (network); porer send e Go `turnstile_failed` dey.
    }
  }

  @override
  Component build(BuildContext context) => const Component.empty();
}
