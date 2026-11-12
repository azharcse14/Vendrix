import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:shared/shared.dart';

import 'price.dart';
import 'responsive_image.dart';
import 'site.dart';

/// List er product (home, category, search): chobi, naam, dam, stock na thakle lekha.
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
    return a(
      href: site.href('/p/${Uri.encodeComponent(product.slug)}'),
      classes: 'block overflow-hidden rounded border border-line bg-surface',
      [
        ResponsiveImage(
          product.image,
          alt: product.name,
          sizes: '(min-width: 1024px) 25vw, (min-width: 768px) 33vw, 50vw',
          lcp: lcp,
        ),
        div(classes: 'space-y-1 p-3', [
          h3(classes: 'line-clamp-2 font-body text-base', [.text(product.name)]),
          Price(product.price, compareAt: product.compareAtPrice, perKg: product.variableWeight),
          if (!product.inStock) p(classes: 'text-sm text-muted', [.text(site.t('product.out_of_stock'))]),
        ]),
      ],
    );
  }
}
