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
  final token = validSid(cookies);
  final key = token == null ? validCartKey(cookies) : null;
  if (token == null && key == null) return null;
  final api = Site.of(context).catalog.api;
  return ApiClient(api.baseUrl, api.client, locale: api.locale, headers: api.headers, token: token, cartKey: key);
}

/// Login e pathano (10.11 item 5, 10.12 item 7): `303 /login?next=<path>`; `__Host-sid` cookie thakle (expire ba
/// revoke) `Max-Age=0`, na hole protiti page e `401`.
Component toLogin(BuildContext context, String path) {
  final site = Site.of(context);
  context.setHeader('location', '${site.href('/login')}?${Uri(queryParameters: {'next': path}).query}');
  if (requestCookies(context).containsKey('__Host-sid')) {
    context.setHeader('set-cookie', '__Host-sid=; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=0');
  }
  context.setStatusCode(303);
  return const Component.empty();
}

/// Ek request er badge count (`GET /api/cart/count`, 7.4 v1.31): normalize/notice na, tai onno page e
/// `item_removed` harabe na. Layout bosay, null = cookie nai.
class CartLoad extends InheritedComponent {
  /// Count (bhul hole error).
  final Future<int>? count;

  /// CartLoad.
  const CartLoad({required this.count, required super.child, super.key});

  /// Upore thaka [CartLoad] er count.
  static Future<int>? of(BuildContext context) => context.dependOnInheritedComponentOfExactType<CartLoad>()?.count;

  @override
  bool updateShouldNotify(CartLoad oldComponent) => false;
}

const _empty = Cart([], 0, 0, [], null);

/// `/cart` (10.1, 10.5, 10.12 item 4, 6): SSR, `@client` na. Protiti line e JS chhara qty form
/// (`POST /_/cart/items/{id}` → PATCH) ar bad form (`.../delete`), `303` e page abar; dam, line total ar subtotal
/// API er (Rule 1). Item thakle checkout link (10.13).
class CartPage extends AsyncStatelessComponent {
  /// URL query (BFF er `error`).
  final Map<String, String> query;

  /// CartPage.
  const CartPage(this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    var cart = _empty;
    // Notice (`item_removed`, `quantity_reduced`) shudhu ei `GET` e (10.12 item 5).
    if (personalApi(context) case final api?) {
      try {
        cart = await CartRepository(api).cart();
      } on ApiError catch (e) {
        if (e.status == 401) return toLogin(context, site.href('/cart'));
        return pageError(context, e);
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
                  if (approx) span(classes: 'text-sm text-muted', [.text(' ${site.t('cart.approx')}')]),
                ]),
              ]),
              a(
                href: site.href('/checkout'),
                classes: 'mt-3 flex min-h-11 items-center justify-center rounded bg-primary px-4 text-on-primary',
                [.text(site.t('cart.checkout'))],
              ),
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
          if (x.variableWeight && x.inStock) span(classes: 'text-sm text-muted', [.text(' ${site.t('cart.approx')}')]),
        ]),
      ]),
    ]);
  }
}
