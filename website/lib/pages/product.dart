import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/add_to_cart.dart';
import '../components/gallery.dart';
import '../components/plain_text.dart';
import '../components/site.dart';
import '../components/variant_picker.dart';
import '../components/wishlist_button.dart';
import 'cart.dart';
import 'status.dart';
import '../seo/json_ld.dart';
import '../seo/meta.dart';

/// Product (`/p/{slug}`, 10.1, 10.5): gallery, naam, rating, variant ar dam (API theke, Rule 1), description.
/// Cart e add (10.12 item 4): JS chhara `form#cart-add` (`variant_id` radio/hidden, `quantity`) `POST /_/cart/items`,
/// mobile e bottom nav er bodole niche sticky bar (dam + variant + button), [AddToCart] JS e. Heart: guest e
/// shuru te off (`wl` JS e, page public cache), login e `GET /api/wishlist` theke (page already `private`).
class ProductPage extends AsyncStatelessComponent {
  /// URL er slug.
  final String slug;

  /// URL query (JS chhara add er BFF `error`).
  final Map<String, String> query;

  /// ProductPage.
  const ProductPage(this.slug, {this.query = const {}, super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final api = personalApi(context);
    // Product er shathe parallel; bhul hole heart off (page bhange na).
    final wished = api?.token == null
        ? null
        : WishlistRepository(api!).wishlist().then((l) => {for (final c in l) c.id}, onError: (_) => <int>{});
    final Product product;
    try {
      product = await site.catalog.product(slug);
    } catch (e) {
      return pageError(context, e);
    }
    final saved = (await wished)?.contains(product.id) ?? false;
    final variants = product.variants;
    final selected = defaultVariant(variants);

    final seo = product.seo;
    final url = '${site.origin}${site.href('/p/${Uri.encodeComponent(product.slug)}')}';
    final images = [for (final im in product.images) ?site.images.url(im.storageKey, 1280)];
    return div(classes: 'px-4 py-4', [
      SeoHead(
        title: seo['title'] is String && (seo['title'] as String).isNotEmpty ? seo['title'] as String : product.name,
        description: seo['description'] is String && (seo['description'] as String).isNotEmpty
            ? seo['description'] as String
            : product.description,
        imageKey: product.images.firstOrNull?.storageKey,
        type: 'product',
        price: selected?.price,
        data: productJsonLd(product, url: url, images: images),
      ),
      div(classes: 'md:grid md:grid-cols-2 md:gap-8', [
        div(classes: '-mx-4 md:mx-0', [Gallery(product.images, alt: product.name)]),
        div(classes: 'mt-4 space-y-4 md:mt-0', [
          h1([.text(product.name)]),
          if (product.reviewCount > 0)
            p(classes: 'text-sm text-muted', [
              span(attributes: {'aria-hidden': 'true'}, [.text('★ ')]),
              .text(
                site.translator.plural('product.rating', product.reviewCount, {
                  'rating': site.rating(product.rating),
                  'count': site.number(product.reviewCount),
                }),
              ),
            ]),
          if (selected == null)
            p(classes: 'text-lg text-muted', [.text(site.t('product.out_of_stock'))])
          else
            _cartForm(site, product, selected),
          wishlistToggle(site, product.id, saved: saved),
          WishlistButton(loggedIn: api?.token != null),
          if (product.description.trim().isNotEmpty)
            section([
              h2(classes: 'mb-2 text-lg', [.text(site.t('product.description'))]),
              ...plainText(product.description),
            ]),
        ]),
      ]),
    ]);
  }

  Component _cartForm(Site site, Product product, Variant selected) {
    final variants = product.variants;
    final error = query['error'];
    const step = 'min-h-11 min-w-11 rounded border border-line';
    return form(id: 'cart-add', action: '/_/cart/items', method: FormMethod.post, classes: 'space-y-4', [
      div(classes: 'text-lg', [VariantPrices(variants, selected, perKg: product.variableWeight)]),
      if (variants.length > 1)
        VariantPicker(variants, selected)
      else
        input(type: InputType.hidden, name: 'variant_id', value: '${selected.id}'),
      label(classes: 'block space-y-1', [
        span(classes: 'text-sm', [.text(site.t('cart.quantity'))]),
        div(classes: 'flex gap-2', [
          // Shudhu JS e (AddToCart dekhay); keyboard e input-i (tai tab e na).
          button(
            type: ButtonType.button,
            classes: step,
            attributes: {'data-qty-step': '-1', 'hidden': '', 'aria-hidden': 'true', 'tabindex': '-1'},
            [.text('−')],
          ),
          input(
            type: InputType.number,
            name: 'quantity',
            value: '1',
            classes: 'min-h-11 w-20 rounded border border-line bg-page px-3 text-base',
            attributes: {'min': '1', 'max': '50', 'inputmode': 'numeric', 'required': ''},
          ),
          button(
            type: ButtonType.button,
            classes: step,
            attributes: {'data-qty-step': '1', 'hidden': '', 'aria-hidden': 'true', 'tabindex': '-1'},
            [.text('+')],
          ),
        ]),
      ]),
      p(
        id: 'cart-add-error',
        classes: 'rounded border border-line bg-tint p-3 text-sm',
        attributes: {'role': 'alert', if (error == null) 'hidden': ''},
        [if (error != null) .text(site.error(error))],
      ),
      p(
        id: 'cart-add-status',
        classes: 'rounded border border-line bg-tint p-3 text-sm',
        attributes: {'role': 'status', 'hidden': ''},
        [
          a(href: site.href('/cart'), classes: 'underline', [.text(site.t('cart.added'))]),
        ],
      ),
      div(
        attributes: {'hidden': ''},
        [
          for (final code in _errors) span(attributes: {'data-cart-error': code}, [.text(site.error(code))]),
        ],
      ),
      // Mobile sticky bar (10.5 niyom 1; Layout ei page e bottom nav dekhay na), desktop e button inline.
      div(
        classes:
            'fixed inset-x-0 bottom-0 z-20 flex min-h-[calc(3.5rem+env(safe-area-inset-bottom))] items-center gap-3 '
            'border-t border-line bg-surface px-4 pb-[env(safe-area-inset-bottom)] '
            'md:static md:z-auto md:min-h-0 md:border-0 md:bg-transparent md:p-0',
        [
          div(classes: 'flex-1 md:hidden', [
            VariantPrices(variants, selected, perKg: product.variableWeight, withLabel: true),
          ]),
          button(type: ButtonType.submit, classes: 'min-h-11 rounded bg-primary px-6 text-on-primary', [
            .text(site.t('cart.add')),
          ]),
        ],
      ),
      AddToCart(digits: [for (var i = 0; i < 10; i++) site.number(i)].join()),
    ]);
  }
}

// Add e ashte pare emon code; lekha server e, client shudhu dekhay.
const _errors = [
  'out_of_stock',
  'cart_full',
  'not_found',
  'validation_failed',
  'rate_limited',
  'forbidden',
  'network_error',
  'timeout',
  'internal',
];
