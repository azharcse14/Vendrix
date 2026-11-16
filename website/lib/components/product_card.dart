import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:shared/shared.dart';

import 'price.dart';
import 'responsive_image.dart';
import 'site.dart';

/// List er product (home, category, search, wishlist): 1:1 chobi, discount ar stock shesh badge, naam (`h2`, page er
/// `h1` er por; 10.7), dam.
class ProductCardView extends StatelessComponent {
  /// API er card.
  final ProductCard product;

  /// Prothom screen er chobi (LCP).
  final bool lcp;

  /// ProductCardView.
  const ProductCardView(this.product, {this.lcp = false, super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final off = discountPercent(product.price, product.compareAtPrice);
    return a(
      href: site.href('/p/${Uri.encodeComponent(product.slug)}'),
      classes: 'block overflow-hidden rounded border border-line bg-surface',
      [
        div(classes: 'relative', [
          // 1:1; stock shesh hole chobi halka, badge e lekha (rong-er upor nirbhor na, 10.7).
          div(classes: 'aspect-square [&>*]:h-full${product.inStock ? '' : ' opacity-60'}', [
            ResponsiveImage(
              product.image,
              alt: product.name,
              sizes: '(min-width: 1024px) 25vw, (min-width: 768px) 33vw, 50vw',
              lcp: lcp,
            ),
          ]),
          if (off != null || !product.inStock)
            div(classes: 'absolute left-2 top-2 flex flex-wrap gap-1', [
              if (off != null)
                span(classes: 'badge badge-accent', [
                  .text(site.t('product.discount', {'percent': site.number(off)})),
                ]),
              if (!product.inStock) span(classes: 'badge badge-danger', [.text(site.t('product.out_of_stock'))]),
            ]),
        ]),
        div(classes: 'space-y-1 p-3', [
          h2(classes: 'line-clamp-2 font-body text-base font-normal', [.text(product.name)]),
          Price(product.price, compareAt: product.compareAtPrice, perKg: product.variableWeight),
        ]),
      ],
    );
  }
}
