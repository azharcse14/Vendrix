import 'dart:convert';
import 'dart:math';

import 'package:http/http.dart' as http;
import 'package:jaspr/jaspr.dart';
import 'package:shared/api/api_client.dart';
import 'package:shared/format/money.dart' show formatTakaDigits;
import 'package:universal_web/js_interop.dart';
import 'package:universal_web/web.dart' as web;

import 'wishlist_button.dart' show loginHref;

// Turnstile er token ekbar e shesh, tai protiti submit er por notun widget token.
@JS('turnstile.reset')
external void _turnstileReset();

const _base32 = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ234567';

/// `Idempotency-Key` (10.13 item 1): 128 bit random (`Random.secure` browser e `crypto.getRandomValues`), base32 26
/// okkhor.
String newIdempotencyKey() {
  final r = Random.secure();
  final out = StringBuffer();
  var bits = 0, n = 0;
  for (var i = 0; i < 16; i++) {
    bits = (bits << 8) | r.nextInt(256);
    n += 8;
    while (n >= 5) {
      n -= 5;
      out.write(_base32[(bits >> n) & 31]);
    }
    bits &= (1 << n) - 1;
  }
  if (n > 0) out.write(_base32[(bits << (5 - n)) & 31]);
  return out.toString();
}

/// OTP code 6 ASCII digit (8.2). Khali code e submit hole `otp_code` jay na, Go abar Turnstile chay.
bool otpCodeOk(String code) => RegExp(r'^[0-9]{6}$').hasMatch(code);

/// Ekoi body (hash er field, 8.6 step 1) er retry (network, OTP) e ekoi key; body bodlale notun key.
class IdempotencyKeys {
  String? _body, _key;

  /// [hashed] er key.
  String keyFor(Map<String, Object?> hashed) {
    final b = jsonEncode(hashed);
    if (b != _body) {
      _body = b;
      _key = newIdempotencyKey();
    }
    return _key!;
  }
}

/// `/checkout` er `form#checkout` (10.13 item 1): district/area/coupon/payment/phone bodlale `/_/checkout/quote`
/// (total shudhu quote theke, Rule 1), submit JSON e `/_/checkout` (`Idempotency-Key`), `409 otp_required` e
/// `#checkout-otp`, `price_changed` e notun total, `201` e `<orderBase><token>/done`. Lekha server e render
/// (`[data-error]`, `[data-otp-text]`); client er data shudhu digit ar path.
@client
class CheckoutForm extends StatefulComponent {
  /// `0`–`9` shop er digit e.
  final String digits;

  /// `/order/` (bhasha prefix shoho).
  final String orderBase;

  /// CheckoutForm.
  const CheckoutForm({required this.digits, required this.orderBase, super.key});

  @override
  State<CheckoutForm> createState() => _CheckoutFormState();
}

class _CheckoutFormState extends State<CheckoutForm> {
  late final ApiClient _bff = ApiClient(Uri.base, http.Client());
  final _keys = IdempotencyKeys();
  web.HTMLFormElement? _form;
  int? _total;
  var _quoteSeq = 0;
  var _busy = false;

  @override
  void initState() {
    super.initState();
    if (!kIsWeb) return;
    final form = _form = web.document.getElementById('checkout') as web.HTMLFormElement?;
    if (form == null) return;
    void on(String selector, void Function() f) {
      final list = form.querySelectorAll(selector);
      for (var i = 0; i < list.length; i++) {
        web.EventStreamProviders.changeEvent.forTarget(list.item(i) as web.Element).listen((_) => f());
      }
    }

    on('[name="address.district"]', () {
      _areas();
      _quote();
    });
    on('select[data-district]', () {
      _areas();
      _quote();
    });
    on('[name="area_other"], [name="coupon_code"], [name="customer.phone"], [name="payment_method"]', _quote);
    on('[name="other_recipient"]', () {
      final on = _checked('other_recipient');
      final box = form.querySelector('#checkout-recipient');
      box?.toggleAttribute('hidden', !on);
      final inputs = box?.querySelectorAll('input');
      for (var i = 0; i < (inputs?.length ?? 0); i++) {
        (inputs!.item(i) as web.Element).toggleAttribute('required', on);
      }
    });
    final resend = form.querySelector('[data-otp-resend]');
    if (resend != null) web.EventStreamProviders.clickEvent.forTarget(resend).listen((_) => _submit(resend: true));
    web.EventStreamProviders.submitEvent.forTarget(form).listen((e) {
      e.preventDefault();
      _submit();
    });
    _areas();
    _quote();
  }

  web.Element? _el(String selector) => _form?.querySelector(selector);

  // Disabled (onno district er) field bad.
  String _v(String name) => ((_el('[name="$name"]:not([disabled])') as web.HTMLInputElement?)?.value ?? '').trim();

  bool _checked(String name) => (_el('[name="$name"]') as web.HTMLInputElement?)?.checked ?? false;

  String get _district => _v('address.district');

  web.HTMLSelectElement? get _areaSelect => _el('select[data-district]:not([disabled])') as web.HTMLSelectElement?;

  // Area: district er select; "Onno area" (`''`) hole free text (8.19 item 2).
  String get _area => _areaSelect?.value == '' ? _v('area_other') : (_areaSelect?.value ?? '');

  String get _payment => (_el('[name="payment_method"]:checked') as web.HTMLInputElement?)?.value ?? '';

  // Ei district er area select dekhay, baki lukano + disabled (submit e jay na).
  void _areas() {
    final list = _form?.querySelectorAll('select[data-district]');
    for (var i = 0; i < (list?.length ?? 0); i++) {
      final s = list!.item(i) as web.HTMLSelectElement;
      final off = s.getAttribute('data-district') != _district;
      s.toggleAttribute('hidden', off);
      s.disabled = off;
    }
    final sel = _areaSelect;
    final other = sel?.value == '';
    _el('#checkout-area-other')?.toggleAttribute('hidden', !other);
    // Delivery er shomoy: lekha server bosiyeche (10.6).
    final pick = sel == null || sel.selectedIndex < 0 ? null : sel.options.item(sel.selectedIndex);
    _el('#checkout-delivery-time')?.textContent = pick?.getAttribute('data-delivery') ?? '';
    (_el('[name="area_other"]') as web.HTMLInputElement?)?.required = other;
  }

  Future<void> _quote() async {
    final seq = ++_quoteSeq;
    _setTotal(null);
    if (_v('customer.phone').isEmpty || _area.isEmpty) return;
    try {
      final q = await _bff.send(
        'POST',
        '/_/checkout/quote',
        body: {
          'coupon_code': _v('coupon_code'),
          'district': _district,
          'area': _area,
          'payment_method': _payment,
          'phone': _v('customer.phone'),
        },
      );
      if (seq != _quoteSeq || q is! Map) return;
      _show(q);
      _problems([for (final p in q['problems'] as List? ?? const []) '$p']);
      _error(null);
    } on ApiError catch (e) {
      if (seq != _quoteSeq) return;
      if (e.status == 401) return web.window.location.assign(loginHref(web.window.location.pathname));
      _problems(const []);
      _error(e.code);
    }
  }

  // Quote ba `price_changed` er `details` er taka.
  void _show(Map<dynamic, dynamic> q) {
    for (final k in const ['subtotal', 'discount', 'delivery_charge', 'total']) {
      final v = q[k];
      _el('[data-quote="$k"]')?.textContent = v is int ? formatTakaDigits(v, component.digits) : '';
    }
    _setTotal(q['total'] is int ? q['total'] as int : null);
  }

  void _setTotal(int? total) {
    _total = total;
    (_el('button[type="submit"]') as web.HTMLButtonElement?)?.disabled = total == null;
  }

  void _problems(List<String> codes) {
    final box = _el('#checkout-problems');
    if (box == null) return;
    box.textContent = [for (final c in codes) _text(c)].join('\n');
    box.toggleAttribute('hidden', codes.isEmpty);
  }

  Map<String, Object?> _hashed() {
    final email = _v('customer.email');
    return {
      'customer': {'name': _v('customer.name'), 'phone': _v('customer.phone'), 'email': email.isEmpty ? null : email},
      'recipient': _checked('other_recipient') ? {'name': _v('recipient.name'), 'phone': _v('recipient.phone')} : null,
      'address': {'district': _district, 'area': _area, 'line': _v('address.line')},
      'note': _v('note'),
      'coupon_code': _v('coupon_code'),
      'payment_method': _payment,
      'expected_total': _total,
      'consent': _checked('consent'),
    };
  }

  Future<void> _submit({bool resend = false}) async {
    if (_busy || _total == null) return;
    _error(null);
    _invalid(const []);
    final otpBox = _el('#checkout-otp');
    final otp = otpBox != null && !otpBox.hasAttribute('hidden') && !resend ? _v('otp_code') : null;
    if (otp != null && !otpCodeOk(otp)) {
      _invalid(const ['otp_code']);
      _error('otp_required');
      return;
    }
    _busy = true;
    final hashed = _hashed();
    try {
      final res = await _bff.send(
        'POST',
        '/_/checkout',
        body: {
          ...hashed,
          'website': _v('website'),
          // OTP er submit e Turnstile lage na (8.6 step 3).
          'turnstile_token': otp == null ? _v('turnstile_token') : '',
          'otp_code': ?otp,
        },
        headers: {'Idempotency-Key': _keys.keyFor(hashed)},
      );
      if (res case {'public_token': final String token} when RegExp(r'^[A-Za-z0-9_-]+$').hasMatch(token)) {
        // _busy thake jate abar submit na hoy.
        web.window.location.assign('${component.orderBase}$token/done');
        return;
      }
      _error('internal');
    } on ApiError catch (e) {
      if (e.status == 401) return web.window.location.assign(loginHref(web.window.location.pathname));
      switch (e.code) {
        case 'otp_required':
          final text = _el('[data-otp-text]')?.textContent ?? '';
          _el('#checkout-otp-text')?.textContent = text.replaceAll('{phone}', '${e.details['phone_masked'] ?? ''}');
          otpBox?.removeAttribute('hidden');
          final code = _el('[name="otp_code"]') as web.HTMLInputElement?;
          code?.value = '';
          code?.focus();
        case 'price_changed':
          // Notun total → body bodlay → notun key (8.6 step 1); customer dekhe abar submit kore.
          _show(e.details);
          _error(e.code);
        case 'validation_failed':
          _invalid(e.fields.keys.toList());
          _error(e.code);
        default:
          _error(e.code);
      }
    } finally {
      if (otp == null) _resetTurnstile();
    }
    _busy = false;
  }

  // Bhul field e `aria-invalid`, prothom tay focus.
  void _invalid(List<String> fields) {
    final old = _form?.querySelectorAll('[aria-invalid]');
    for (var i = 0; i < (old?.length ?? 0); i++) {
      (old!.item(i) as web.Element).removeAttribute('aria-invalid');
    }
    for (final f in fields.reversed) {
      if (!RegExp(r'^[a-z_.]+$').hasMatch(f)) continue;
      final name = f == 'address.area' && _areaSelect?.value == '' ? 'area_other' : f;
      final el = _el('[name="$name"]:not([disabled])') as web.HTMLElement?;
      el?.setAttribute('aria-invalid', 'true');
      el?.focus();
    }
  }

  // Server er render kora lekha; ochena code e `internal`.
  String _text(String code) {
    final known = RegExp(r'^[a-z_]+$').hasMatch(code) ? code : 'internal';
    return (web.document.querySelector('[data-error="$known"]') ??
                web.document.querySelector('[data-error="internal"]'))
            ?.textContent ??
        '';
  }

  void _error(String? code) {
    final box = web.document.getElementById('checkout-error');
    if (box == null) return;
    box.textContent = code == null ? '' : _text(code);
    box.toggleAttribute('hidden', code == null);
  }

  void _resetTurnstile() {
    try {
      _turnstileReset();
    } catch (_) {
      // Script ashe ni (network); porer submit e Go `turnstile_failed` dey.
    }
  }

  @override
  Component build(BuildContext context) => const Component.empty();
}
