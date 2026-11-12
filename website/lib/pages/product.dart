import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/gallery.dart';
import '../components/plain_text.dart';
import '../components/site.dart';
import '../components/variant_picker.dart';
import 'status.dart';

/// Product (`/p/{slug}`, 10.1, 10.5): gallery, naam, rating, variant ar dam (API theke, Rule 1), description.
/// Mobile e bottom nav er bodole niche sticky bar (dam + variant); "Cart e add" M5 e (10.10 item 6).
class ProductPage extends AsyncStatelessComponent {
  /// URL er slug.
  final String slug;

  /// ProductPage.
  const ProductPage(this.slug, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final Product product;
    try {
      product = await site.catalog.product(slug);
    } catch (e) {
      return pageError(context, e);
    }
    final variants = product.variants;
    final selected = defaultVariant(variants);
    final prices = selected == null
        ? p(classes: 'text-muted', [.text(site.t('product.out_of_stock'))])
        : VariantPrices(variants, selected, perKg: product.variableWeight);

    return div(classes: 'px-4 py-4', [
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
          div(classes: 'text-lg', [prices]),
          if (selected != null && variants.length > 1) VariantPicker(variants, selected),
          if (product.description.trim().isNotEmpty)
            section([
              h2(classes: 'mb-2 text-lg', [.text(site.t('product.description'))]),
              ...plainText(product.description),
            ]),
        ]),
      ]),
      // Mobile sticky bar (10.5 niyom 1); Layout ei page e bottom nav dekhay na.
      if (selected != null)
        div(
          classes:
              'fixed inset-x-0 bottom-0 z-20 flex min-h-[calc(3.5rem+env(safe-area-inset-bottom))] items-center '
              'border-t border-line bg-surface px-4 pb-[env(safe-area-inset-bottom)] md:hidden',
          [VariantPrices(variants, selected, perKg: product.variableWeight, withLabel: true)],
        ),
    ]);
  }
}
