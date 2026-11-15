import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';

import '../components/login_form.dart';
import '../components/site.dart';
import '../seo/meta.dart';

/// Login er por jawar path (10.11 item 1): shudhu `/` diye shuru; `//`, `\` ba control okkhor (browser tab/newline
/// muche `//` banay) hole [fallback]. Open redirect na.
String safeNext(String? next, String fallback) {
  if (next == null || !next.startsWith('/') || next.startsWith('//') || next.contains(r'\')) return fallback;
  if (next.codeUnits.any((c) => c < 0x20 || c == 0x7f)) return fallback;
  return next;
}

// Send/verify e ashte pare emon code; lekha server e render, client shudhu dekhay.
const _errors = [
  'invalid_phone',
  'turnstile_failed',
  'phone_blocked',
  'otp_invalid',
  'otp_too_many_attempts',
  'rate_limited',
  'validation_failed',
  'forbidden',
  'network_error',
  'timeout',
  'internal',
];

const _field = 'min-h-11 w-full rounded border border-line bg-page px-3 text-base';
const _button = 'min-h-11 w-full rounded bg-primary px-4 text-on-primary';

/// `/login` (10.1, 10.11): phone → Turnstile → code; kaj [LoginForm] e (JS lage, Turnstile JS chhara chole na).
/// Form er `action` BFF e, tai JS chhara submit e `?error=` niye phire ashe.
class LoginPage extends StatelessComponent {
  /// URL query (`next`, `error`).
  final Map<String, String> query;

  /// LoginPage.
  const LoginPage(this.query, {super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final error = query['error'];
    // Input label er bhetore (10.7).
    Component field(String text, Component input) => label(classes: 'block space-y-1', [
      span(classes: 'text-sm', [.text(text)]),
      input,
    ]);
    return div(classes: 'mx-auto max-w-sm space-y-4 px-4 py-8', [
      SeoHead(title: site.t('login.title'), noindex: true),
      Document.head(
        children: [script(src: 'https://challenges.cloudflare.com/turnstile/v0/api.js', async: true, defer: true)],
      ),
      h1(classes: 'font-heading text-2xl', [.text(site.t('login.title'))]),
      Component.element(
        tag: 'noscript',
        children: [
          p([.text(site.t('login.noscript'))]),
        ],
      ),
      form(id: 'login-phone', method: FormMethod.post, action: '/_/auth/otp/send', classes: 'space-y-4', [
        input(type: InputType.hidden, name: 'purpose', value: 'login'),
        field(
          site.t('login.phone'),
          input(
            type: InputType.tel,
            name: 'phone',
            classes: _field,
            attributes: {'inputmode': 'numeric', 'autocomplete': 'tel', 'required': ''},
          ),
        ),
        div(
          classes: 'cf-turnstile',
          attributes: {
            'data-sitekey': site.turnstileSiteKey,
            'data-response-field-name': 'turnstile_token',
            'data-language': site.locale,
          },
          [],
        ),
        button(type: ButtonType.submit, classes: _button, [.text(site.t('login.send'))]),
      ]),
      form(
        id: 'login-code',
        method: FormMethod.post,
        action: '/_/auth/otp/verify',
        classes: 'space-y-4',
        attributes: {'hidden': ''},
        [
          p([.text(site.t('login.code_sent'))]),
          field(
            site.t('login.code'),
            input(
              type: InputType.text,
              name: 'code',
              classes: _field,
              attributes: {'inputmode': 'numeric', 'autocomplete': 'one-time-code', 'required': ''},
            ),
          ),
          button(type: ButtonType.submit, classes: _button, [.text(site.t('login.verify'))]),
          button(
            type: ButtonType.button,
            classes: 'min-h-11 w-full underline',
            attributes: {'data-login-back': ''},
            [.text(site.t('login.change_phone'))],
          ),
        ],
      ),
      p(
        id: 'login-error',
        classes: 'rounded border border-line bg-tint p-3 text-sm',
        attributes: {'role': 'alert', if (error == null) 'hidden': ''},
        [if (error != null) .text(site.error(error))],
      ),
      div(
        attributes: {'hidden': ''},
        [
          for (final code in _errors) span(attributes: {'data-error': code}, [.text(site.error(code))]),
        ],
      ),
      LoginForm(next: safeNext(query['next'], site.href('/account'))),
    ]);
  }
}
