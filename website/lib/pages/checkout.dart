import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/checkout_form.dart';
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

const _field = 'min-h-11 w-full rounded border border-line bg-page px-3 text-base';

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
      return div(classes: 'px-4 py-6', [
        ...head,
        p(classes: 'text-muted', [.text(site.t('cart.empty'))]),
        a(href: site.href('/cart'), classes: 'mt-4 inline-flex min-h-11 items-center underline', [
          .text(site.t('cart.title')),
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
        classes: _field,
        attributes: attrs,
      ),
    );
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
          p([.text(site.t('checkout.noscript'))]),
        ],
      ),
      form(
        id: 'checkout',
        method: FormMethod.post,
        action: '/_/checkout',
        classes: 'lg:grid lg:grid-cols-3 lg:items-start lg:gap-8',
        [
          div(classes: 'space-y-4 lg:col-span-2', [
            field(
              site.t('checkout.name'),
              input(
                type: InputType.text,
                name: 'customer.name',
                value: me?.name,
                classes: _field,
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
                classes: _field,
                attributes: {...tel, 'required': '', if (me != null) 'readonly': ''},
              ),
            ),
            field(
              site.t('checkout.email'),
              input(
                type: InputType.email,
                name: 'customer.email',
                value: me?.email,
                classes: _field,
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
            field(
              site.t('checkout.district'),
              select(
                name: 'address.district',
                classes: _field,
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
                  classes: _field,
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
                classes: _field,
                attributes: {'maxlength': '100'},
              ),
              id: 'checkout-area-other',
              hidden: first == null || first.areas.isNotEmpty,
            ),
            // Order er age delivery er shomoy (10.6, 12.2); client area bachai onujayi option er lekha bosay.
            p(
              id: 'checkout-delivery-time',
              classes: 'text-sm',
              attributes: {'aria-live': 'polite'},
              [.text(delivery(first?.areas.firstOrNull?.days ?? first?.districtWideDays))],
            ),
            field(
              site.t('checkout.address'),
              textarea(
                name: 'address.line',
                classes: '$_field py-2',
                attributes: {'autocomplete': 'street-address', 'minlength': '5', 'maxlength': '500', 'required': ''},
                [],
              ),
            ),
            field(
              site.t('checkout.note'),
              textarea(name: 'note', classes: '$_field py-2', attributes: {'maxlength': '500'}, []),
            ),
            text('coupon_code', site.t('checkout.coupon'), attrs: const {'maxlength': '32', 'autocomplete': 'off'}),
            fieldset(classes: 'space-y-2', [
              legend(classes: 'text-sm', [.text(site.t('checkout.payment'))]),
              for (final m in methods)
                label(classes: 'flex min-h-11 items-center gap-2', [
                  input(
                    type: InputType.radio,
                    name: 'payment_method',
                    value: m,
                    classes: 'size-5',
                    attributes: {if (m == methods.first) 'checked': ''},
                  ),
                  span([.text(site.t('order.payment.$m'))]),
                ]),
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
            classes: 'mt-6 space-y-4 lg:sticky lg:top-4 lg:mt-0 lg:rounded lg:border lg:border-line lg:p-4',
            attributes: {'aria-label': site.t('checkout.total')},
            [
              ul(classes: 'divide-y divide-line text-sm', [
                for (final x in cart.items)
                  li(classes: 'flex justify-between gap-2 py-2', [
                    span([
                      .text(
                        '${x.name}${x.variantLabel.isEmpty ? '' : ' (${x.variantLabel})'} × ${site.number(x.quantity)}',
                      ),
                    ]),
                    Price(x.lineTotal),
                  ]),
              ]),
              dl(
                classes: 'space-y-1',
                attributes: {'aria-live': 'polite'},
                [
                  for (final k in const ['subtotal', 'discount', 'delivery_charge', 'total'])
                    div(classes: k == 'total' ? 'flex justify-between text-lg font-medium' : 'flex justify-between', [
                      dt([.text(site.t('checkout.$k'))]),
                      dd(attributes: {'data-quote': k}, []),
                    ]),
                ],
              ),
              p(
                id: 'checkout-problems',
                classes: 'whitespace-pre-line rounded border border-line bg-tint p-3 text-sm',
                attributes: {'role': 'status', 'hidden': ''},
                [],
              ),
              label(classes: 'flex min-h-11 items-center gap-2', [
                input(type: InputType.checkbox, name: 'consent', classes: 'size-5', attributes: {'required': ''}),
                span([.text(site.t('checkout.consent'))]),
              ]),
              fieldset(
                id: 'checkout-otp',
                classes: 'space-y-2',
                attributes: {'hidden': ''},
                [
                  p(id: 'checkout-otp-text', attributes: {'role': 'status'}, []),
                  field(
                    site.t('checkout.otp_code'),
                    input(
                      type: InputType.text,
                      name: 'otp_code',
                      classes: _field,
                      attributes: {'inputmode': 'numeric', 'autocomplete': 'one-time-code'},
                    ),
                  ),
                  button(
                    type: ButtonType.button,
                    classes: 'min-h-11 w-full underline',
                    attributes: {'data-otp-resend': ''},
                    [.text(site.t('checkout.otp_resend'))],
                  ),
                ],
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
              p(
                id: 'checkout-error',
                classes: 'rounded border border-line bg-tint p-3 text-sm',
                attributes: {'role': 'alert', if (error == null) 'hidden': ''},
                [if (error != null) .text(site.error(error))],
              ),
              button(
                type: ButtonType.submit,
                classes: 'min-h-11 w-full rounded bg-primary px-4 text-on-primary disabled:opacity-50',
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
