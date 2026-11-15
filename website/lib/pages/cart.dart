import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/price.dart';
import '../components/site.dart';
import '../seo/meta.dart';
import '../services/bff.dart';
import 'status.dart';

/// Request er cookie. Jaspr er `context.cookies` na: ta `dart:io` `Cookie` diye pore, `wl` er koma (10.12 item 3)
/// te puro page bhange.
Map<String, String> requestCookies(BuildContext context) => parseCookies(context.headers['cookie']);

/// Cookie er session ba cart key diye [ApiClient] (login e shudhu Bearer, 7.4); duitar kono ta na thakle null
/// (API call na, page cache thake, 10.12 item 5).
ApiClient? personalApi(BuildContext context) {
  final cookies = requestCookies(context);
  final sid = cookies['__Host-sid'];
  final token = sid == null || sid.isEmpty ? null : sid;
  final key = token == null ? validCartKey(cookies) : null;
  if (token == null && key == null) return null;
  final api = Site.of(context).catalog.api;
  return ApiClient(api.baseUrl, api.client, locale: api.locale, headers: api.headers, token: token, cartKey: key);
}

/// Ek request er `GET /api/cart` (Layout er badge ar `/cart` ekta-i call: GET notice ekbar dey, 8.4). Layout
/// bosay, null = cookie nai.
class CartLoad extends InheritedComponent {
  /// Cart (bhul hole error).
  final Future<Cart>? cart;

  /// CartLoad.
  const CartLoad({required this.cart, required super.child, super.key});

  /// Upore thaka [CartLoad] er cart.
  static Future<Cart>? of(BuildContext context) => context.dependOnInheritedComponentOfExactType<CartLoad>()?.cart;

  @override
  bool updateShouldNotify(CartLoad oldComponent) => false;
}

const _empty = Cart([], 0, 0, [], null);

/// `/cart` (10.1, 10.5, 10.12 item 4, 6): SSR, `@client` na. Protiti line e JS chhara qty form
/// (`POST /_/cart/items/{id}` → PATCH) ar bad form (`.../delete`), `303` e page abar; dam, line total ar subtotal
/// API er (Rule 1). Checkout button M6 e.
class CartPage extends AsyncStatelessComponent {
  /// URL query (BFF er `error`).
  final Map<String, String> query;

  /// CartPage.
  const CartPage(this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    var cart = _empty;
    if (CartLoad.of(context) case final load?) {
      try {
        cart = await load;
      } on ApiError catch (e) {
        // Session expire: login kora cart dekhano jay na, khali (login link header e).
        if (e.status != 401) return pageError(context, e);
      } catch (e) {
        return pageError(context, e);
      }
    }
    final error = query['error'];
    final approx = cart.items.any((x) => x.variableWeight && x.inStock);
    return div(classes: 'px-4 py-6', [
      SeoHead(title: site.t('cart.title'), noindex: true),
      h1(classes: 'mb-4 font-heading text-2xl', [.text(site.t('cart.title'))]),
      if (error != null)
        p(
          classes: 'mb-4 rounded border border-line bg-tint p-3 text-sm',
          attributes: {'role': 'alert'},
          [.text(site.error(error))],
        ),
      if (cart.notices.isNotEmpty)
        div(
          classes: 'mb-4 space-y-1 rounded border border-line bg-tint p-3 text-sm',
          attributes: {'role': 'status'},
          [
            for (final n in cart.notices)
              p([
                .text(site.t('cart.notice.${n.code}', {'name': n.name})),
              ]),
          ],
        ),
      if (cart.items.isEmpty)
        p(classes: 'text-muted', [.text(site.t('cart.empty'))])
      else
        div(classes: 'lg:grid lg:grid-cols-3 lg:items-start lg:gap-8', [
          ul(classes: 'divide-y divide-line lg:col-span-2', [for (final x in cart.items) _Line(x)]),
          // Mobile e niche sticky (bottom nav er upore), desktop e dane (10.5).
          section(
            classes:
                'sticky bottom-[calc(3.5rem+env(safe-area-inset-bottom))] -mx-4 mt-4 border-t border-line bg-surface '
                'p-4 md:bottom-0 lg:bottom-auto lg:top-4 lg:mx-0 lg:mt-0 lg:rounded lg:border',
            attributes: {'aria-label': site.t('cart.subtotal')},
            [
              p(classes: 'text-sm text-muted', [
                .text(site.translator.plural('cart.items', cart.count, {'count': site.number(cart.count)})),
              ]),
              p(classes: 'flex items-baseline justify-between text-lg', [
                span([.text(site.t('cart.subtotal'))]),
                span(classes: 'font-medium', [
                  .text(site.taka(cart.subtotal)),
                  if (approx) span(classes: 'text-sm text-muted', [.text(' (${site.t('cart.approx')})')]),
                ]),
              ]),
            ],
          ),
        ]),
    ]);
  }
}

// Ek line: naam, variant, dam, qty form, bad form, line total.
class _Line extends StatelessComponent {
  final CartItem item;
  const _Line(this.item);

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final x = item;
    final id = 'qty-${x.variantId}';
    return li(classes: 'space-y-2 py-4', [
      a(href: site.href('/p/${Uri.encodeComponent(x.productSlug)}'), classes: 'font-medium underline', [
        .text(x.name),
      ]),
      if (x.variantLabel.isNotEmpty) p(classes: 'text-sm text-muted', [.text(x.variantLabel)]),
      Price(x.unitPrice, compareAt: x.compareAtPrice, perKg: x.variableWeight),
      if (!x.inStock) p(classes: 'text-sm text-muted', [.text(site.t('product.out_of_stock'))]),
      div(classes: 'flex flex-wrap items-end gap-2', [
        form(action: '/_/cart/items/${x.variantId}', method: FormMethod.post, classes: 'flex items-end gap-2', [
          label(classes: 'block space-y-1', [
            span(classes: 'text-sm', [.text(site.t('cart.quantity'))]),
            input(
              id: id,
              type: InputType.number,
              name: 'quantity',
              value: '${x.quantity}',
              classes: 'block min-h-11 w-20 rounded border border-line bg-page px-3 text-base',
              attributes: {'min': '1', 'max': '${x.maxQuantity}', 'inputmode': 'numeric', 'required': ''},
            ),
          ]),
          button(type: ButtonType.submit, classes: 'min-h-11 rounded border border-line px-4', [
            .text(site.t('cart.update')),
          ]),
        ]),
        form(action: '/_/cart/items/${x.variantId}/delete', method: FormMethod.post, [
          button(type: ButtonType.submit, classes: 'min-h-11 px-2 underline', [.text(site.t('cart.remove'))]),
        ]),
        p(classes: 'ml-auto font-medium', [
          .text(site.taka(x.lineTotal)),
          if (x.variableWeight && x.inStock)
            span(classes: 'text-sm text-muted', [.text(' (${site.t('cart.approx')})')]),
        ]),
      ]),
    ]);
  }
}
