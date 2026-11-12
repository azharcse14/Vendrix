import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:shared/shared.dart';

import 'responsive_image.dart';
import 'site.dart';

/// Product er chobi (10.2, 10.5): CSS `scroll-snap` e swipe, JS na; prothom chobi LCP.
class Gallery extends StatelessComponent {
  /// Chobi (sort_order kram e).
  final List<ProductImage> images;

  /// `alt` (product naam, 10.7).
  final String alt;

  /// Gallery.
  const Gallery(this.images, {required this.alt, super.key});

  static const _sizes = '(min-width: 768px) 50vw, 100vw';

  @override
  Component build(BuildContext context) {
    if (images.isEmpty) return ResponsiveImage(null, alt: alt, sizes: _sizes);
    return div(
      classes: 'flex snap-x snap-mandatory overflow-x-auto rounded bg-tint',
      // Keyboard e scroll (10.7).
      attributes: {'role': 'region', 'aria-label': Site.of(context).t('product.images'), 'tabindex': '0'},
      [
        for (final (i, im) in images.indexed)
          div(classes: 'w-full shrink-0 snap-center', [
            ResponsiveImage(im.storageKey, alt: alt, sizes: _sizes, width: im.width, height: im.height, lcp: i == 0),
          ]),
      ],
    );
  }
}
