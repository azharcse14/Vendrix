import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/checkout_form.dart';
import '../components/icon.dart';
import '../components/price.dart';
import '../components/site.dart';
import '../seo/meta.dart';
import 'cart.dart';
import 'status.dart';

// Quote ar order e ashte pare emon code; lekha server e, client shudhu dekhay.
const _errors = [
  'validation_failed',
  'invalid_phone',
  'area_not_served',
  'below_min_order',
  'coupon_invalid',
  'coupon_limit_reached',
  'cart_empty',
  'out_of_stock',
  'vendor_unavailable',
  'price_changed',
  'otp_required',
  'otp_invalid',
  'otp_too_many_attempts',
  'turnstile_failed',
  'phone_blocked',
  'rate_limited',
  'idempotency_key_reused',
  'missing_idempotency_key',
  'forbidden',
  'network_error',
  'timeout',
  'internal',
];

// `text-base` `.input` e-o ache; checkout_test 16px (10.5) class dhore tai rakha.
const _input = 'input text-base';

/// `/checkout` (10.1, 10.6, 10.13 item 1–3): SSR e cart, delivery zone ar (login e) `/api/me`; form er kaj
/// [CheckoutForm] e (JS lage, Turnstile JS chhara chole na). Dam/total shudhu quote theke (Rule 1). Cookie ba item na
/// thakle form na.
class CheckoutPage extends AsyncStatelessComponent {
  /// URL query (BFF er `error`).
  final Map<String, String> query;

  /// CheckoutPage.
  const CheckoutPage(this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final api = personalApi(context);
    Cart? cart;
    List<DeliveryZone> zones = const [];
    User? me;
    if (api != null) {
      try {
        cart = await CartRepository(api).cart();
        if (cart.items.isNotEmpty) {
          final zonesF = CheckoutRepository(api).zones();
          final meF = api.token == null ? null : AccountRepository(api).me();
          zones = await zonesF;
          me = await meF;
        }
      } on ApiError catch (e) {
        if (e.status == 401) return toLogin(context, site.href('/checkout'));
        return pageError(context, e);
      } catch (e) {
        return pageError(context, e);
      }
    }
    final head = [
      SeoHead(title: site.t('checkout.title'), noindex: true),
      h1(classes: 'mb-4 font-heading text-2xl', [.text(site.t('checkout.title'))]),
    ];
    if (cart == null || cart.items.isEmpty) {
      // Empty state (U7 er moto): icon, lekha, cart e link.
      return div(classes: 'px-4 py-6', [
        ...head,
        div(classes: 'flex flex-col items-center gap-3 py-12 text-center', [
          const Icon(Icons.cart, classes: 'size-10 text-muted'),
          p(classes: 'text-muted', [.text(site.t('cart.empty'))]),
          a(href: site.href('/cart'), classes: 'btn btn-secondary', [.text(site.t('cart.title'))]),
        ]),
      ]);
    }

    // Din server er (8.15); niyom ekhane na.
    String delivery(int? days) => days == null ? '' : site.t('checkout.delivery_time', {'days': site.number(days)});
    Component field(String text, Component input, {String? id, bool hidden = false}) => label(
      id: id,
      classes: 'block space-y-1',
      attributes: {if (hidden) 'hidden': ''},
      [
        span(classes: 'text-sm', [.text(text)]),
        input,
      ],
    );
    Component text(
      String name,
      String label, {
      InputType type = InputType.text,
      Map<String, String> attrs = const {},
    }) => field(
      label,
      input(
        type: type,
        name: name,
        classes: _input,
        attributes: attrs,
      ),
    );
    // Numbered section card (plan 5: ek page, step indicator na).
    Component card(int n, String key, List<Component> children, {String? id, bool hidden = false}) => section(
      id: id,
      classes: 'card space-y-4',
      attributes: {if (hidden) 'hidden': ''},
      [
        h2(id: 'checkout-step-$n', classes: 'flex items-center gap-3 text-xl', [
          span(
            classes:
                'inline-flex size-8 shrink-0 items-center justify-center rounded-full bg-primary-soft text-base text-primary',
            [.text(site.number(n))],
          ),
          .text(site.t(key)),
        ]),
        ...children,
      ],
    );
    Component lines() => ul(classes: 'divide-y divide-line text-sm', [
      for (final x in cart!.items)
        li(classes: 'flex justify-between gap-2 py-2', [
          span([
            .text('${x.name}${x.variantLabel.isEmpty ? '' : ' (${x.variantLabel})'} × ${site.number(x.quantity)}'),
          ]),
          Price(x.lineTotal),
        ]),
    ]);
    const tel = {'inputmode': 'numeric', 'autocomplete': 'tel'};
    final methods = site.settings.paymentMethods.isEmpty ? const ['cod'] : site.settings.paymentMethods;
    final first = zones.isEmpty ? null : zones.first;
    final error = query['error'];

    return div(classes: 'px-4 py-6', [
      ...head,
      Document.head(
        children: [script(src: 'https://challenges.cloudflare.com/turnstile/v0/api.js', async: true, defer: true)],
      ),
      Component.element(
        tag: 'noscript',
        children: [
          p(classes: 'notice notice-warning mb-4', [.text(site.t('checkout.noscript'))]),
        ],
      ),
      form(
        id: 'checkout',
        method: FormMethod.post,
        action: '/_/checkout',
        classes: 'lg:grid lg:grid-cols-3 lg:items-start lg:gap-8',
        [
          // Mobile e upore bondho summary (JS chhara); desktop e dan er card e line.
          details(classes: 'card mb-4 lg:hidden', [
            summary(classes: 'flex min-h-11 cursor-pointer items-center justify-between gap-2', [
              span([.text('${site.t('checkout.summary')} (${site.number(cart.items.length)})')]),
              const Icon(Icons.chevronDown),
            ]),
            lines(),
          ]),
          div(classes: 'space-y-4 lg:col-span-2', [
            card(1, 'checkout.contact', [
              field(
                site.t('checkout.name'),
                input(
                  type: InputType.text,
                  name: 'customer.name',
                  value: me?.name,
                  classes: _input,
                  attributes: {'autocomplete': 'name', 'maxlength': '100', 'required': ''},
                ),
              ),
              // Login e account er phone, bodlano jay na (8.19 item 4).
              field(
                site.t('checkout.phone'),
                input(
                  type: InputType.tel,
                  name: 'customer.phone',
                  value: me?.phone,
                  classes: _input,
                  attributes: {...tel, 'required': '', if (me != null) 'readonly': ''},
                ),
              ),
              field(
                site.t('checkout.email'),
                input(
                  type: InputType.email,
                  name: 'customer.email',
                  value: me?.email,
                  classes: _input,
                  attributes: {'autocomplete': 'email', 'maxlength': '254'},
                ),
              ),
              label(classes: 'flex min-h-11 items-center gap-2', [
                input(type: InputType.checkbox, name: 'other_recipient', classes: 'size-5'),
                span([.text(site.t('checkout.other_recipient'))]),
              ]),
              div(
                id: 'checkout-recipient',
                classes: 'space-y-4',
                attributes: {'hidden': ''},
                [
                  text('recipient.name', site.t('checkout.recipient_name'), attrs: const {'maxlength': '100'}),
                  text('recipient.phone', site.t('checkout.recipient_phone'), type: InputType.tel, attrs: tel),
                ],
              ),
            ]),
            card(2, 'checkout.shipping', [
              field(
                site.t('checkout.district'),
                select(
                  name: 'address.district',
                  classes: _input,
                  attributes: {'required': ''},
                  [
                    for (final z in zones) option(value: z.district, [.text(z.district)]),
                  ],
                ),
              ),
              // Protiti district er area select; client district onujayi dekhay (8.19 item 2).
              div(classes: 'block space-y-1', [
                span(id: 'checkout-area-label', classes: 'text-sm', [.text(site.t('checkout.area'))]),
                for (final z in zones)
                  select(
                    name: 'address.area',
                    classes: _input,
                    attributes: {
                      'data-district': z.district,
                      'aria-labelledby': 'checkout-area-label',
                      if (z != first) ...{'hidden': '', 'disabled': ''},
                    },
                    [
                      for (final x in z.areas)
                        option(value: x.area, attributes: {'data-delivery': delivery(x.days)}, [.text(x.name)]),
                      if (z.districtWide)
                        option(
                          value: '',
                          attributes: {'data-delivery': delivery(z.districtWideDays)},
                          [.text(site.t('checkout.area_other'))],
                        ),
                    ],
                  ),
              ]),
              field(
                site.t('checkout.area_other'),
                input(
                  type: InputType.text,
                  name: 'area_other',
                  classes: _input,
                  attributes: {'maxlength': '100'},
                ),
                id: 'checkout-area-other',
                hidden: first == null || first.areas.isNotEmpty,
              ),
              // Order er age delivery er shomoy (10.6, 12.2); client area bachai onujayi option er lekha bosay.
              p(
                id: 'checkout-delivery-time',
                classes: 'text-sm text-muted',
                attributes: {'aria-live': 'polite'},
                [.text(delivery(first?.areas.firstOrNull?.days ?? first?.districtWideDays))],
              ),
              field(
                site.t('checkout.address'),
                textarea(
                  name: 'address.line',
                  classes: _input,
                  attributes: {'autocomplete': 'street-address', 'minlength': '5', 'maxlength': '500', 'required': ''},
                  [],
                ),
              ),
              field(
                site.t('checkout.note'),
                textarea(name: 'note', classes: _input, attributes: {'maxlength': '500'}, []),
              ),
            ]),
            card(3, 'checkout.payment', [
              // Radio card: `.chip:has(:checked)` e bachai (JS chhara).
              fieldset(
                classes: 'grid gap-2 sm:grid-cols-2',
                attributes: {'aria-labelledby': 'checkout-step-3'},
                [
                  for (final m in methods)
                    label(classes: 'chip w-full cursor-pointer rounded px-4 text-base', [
                      input(
                        type: InputType.radio,
                        name: 'payment_method',
                        value: m,
                        classes: 'size-5',
                        attributes: {if (m == methods.first) 'checked': ''},
                      ),
                      span([.text(site.t('order.payment.$m'))]),
                    ]),
                ],
              ),
              text('coupon_code', site.t('checkout.coupon'), attrs: const {'maxlength': '32', 'autocomplete': 'off'}),
            ]),
            card(4, 'checkout.otp_title', id: 'checkout-otp', hidden: true, [
              p(id: 'checkout-otp-text', attributes: {'role': 'status'}, []),
              field(
                site.t('checkout.otp_code'),
                input(
                  type: InputType.text,
                  name: 'otp_code',
                  classes: '$_input text-center font-heading text-2xl tracking-widest',
                  attributes: {'inputmode': 'numeric', 'autocomplete': 'one-time-code'},
                ),
              ),
              button(
                type: ButtonType.button,
                classes: 'btn btn-ghost w-full',
                attributes: {'data-otp-resend': ''},
                [.text(site.t('checkout.otp_resend'))],
              ),
            ]),
            // Honeypot (8.6 step 3): manush dekhe na.
            div(
              classes: 'hidden',
              attributes: {'aria-hidden': 'true'},
              [
                input(
                  type: InputType.text,
                  name: 'website',
                  attributes: {'tabindex': '-1', 'autocomplete': 'off'},
                ),
              ],
            ),
          ]),
          section(
            classes: 'card mt-4 space-y-4 shadow-1 lg:sticky lg:top-4 lg:mt-0',
            attributes: {'aria-label': site.t('checkout.total')},
            [
              div(classes: 'hidden lg:block', [lines()]),
              dl(
                classes: 'space-y-1',
                attributes: {'aria-live': 'polite'},
                [
                  for (final k in const ['subtotal', 'discount', 'delivery_charge', 'total'])
                    div(
                      classes: k == 'total'
                          ? 'flex justify-between border-t border-line pt-2 font-heading text-xl'
                          : 'flex justify-between',
                      [
                        dt([.text(site.t('checkout.$k'))]),
                        dd(attributes: {'data-quote': k}, []),
                      ],
                    ),
                ],
              ),
              // Quote er problem (price_changed shoho): warning (plan 5).
              p(
                id: 'checkout-problems',
                classes: 'notice notice-warning whitespace-pre-line text-sm',
                attributes: {'role': 'status', 'hidden': ''},
                [],
              ),
              label(classes: 'flex min-h-11 items-center gap-2', [
                input(type: InputType.checkbox, name: 'consent', classes: 'size-5', attributes: {'required': ''}),
                span([.text(site.t('checkout.consent'))]),
              ]),
              div(
                classes: 'cf-turnstile',
                attributes: {
                  'data-sitekey': site.turnstileSiteKey,
                  'data-response-field-name': 'turnstile_token',
                  'data-language': site.locale,
                },
                [],
              ),
              p(
                id: 'checkout-error',
                classes: 'notice notice-danger text-sm',
                attributes: {'role': 'alert', if (error == null) 'hidden': ''},
                [if (error != null) .text(site.error(error))],
              ),
              button(
                type: ButtonType.submit,
                classes: 'btn btn-primary w-full',
                attributes: {'disabled': ''},
                [.text(site.t('checkout.submit'))],
              ),
            ],
          ),
          // Client er lekha; form er bhetore, karon CheckoutForm form e khoje.
          div(
            attributes: {'hidden': ''},
            [
              for (final code in _errors) span(attributes: {'data-error': code}, [.text(site.error(code))]),
              span(
                attributes: {'data-otp-text': ''},
                [
                  .text(site.t('checkout.otp_sent', {'phone': '{phone}'})),
                ],
              ),
            ],
          ),
        ],
      ),
      CheckoutForm(
        digits: [for (var i = 0; i < 10; i++) site.number(i)].join(),
        orderBase: site.href('/order/'),
      ),
    ]);
  }
}
