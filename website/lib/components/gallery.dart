import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:shared/shared.dart';

import 'responsive_image.dart';
import 'site.dart';

/// Product er chobi (10.2, 10.5): CSS `scroll-snap` e swipe, JS na; prothom chobi LCP. Desktop e thumbnail row
/// (`#img-<n>` link, JS chhara oi chobi te scroll).
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
    final site = Site.of(context);
    final slides = div(
      classes: 'flex snap-x snap-mandatory overflow-x-auto rounded bg-tint md:rounded-lg',
      // Keyboard e scroll (10.7).
      attributes: {'role': 'region', 'aria-label': site.t('product.images'), 'tabindex': '0'},
      [
        for (final (i, im) in images.indexed)
          div(id: 'img-${i + 1}', classes: 'w-full shrink-0 snap-center', [
            ResponsiveImage(im.storageKey, alt: alt, sizes: _sizes, width: im.width, height: im.height, lcp: i == 0),
          ]),
      ],
    );
    if (images.length < 2) return slides;
    return div([
      slides,
      // Mobile e swipe-i; thumbnail shudhu desktop e. alt khali: boro chobir alt-i ache, link e "chobi n".
      ul(classes: 'mt-3 hidden gap-2 md:flex', [
        for (final (i, im) in images.indexed)
          li([
            a(
              href: '#img-${i + 1}',
              classes: 'block w-16 overflow-hidden rounded-sm border border-line hover:border-primary',
              attributes: {'aria-label': '${site.t('product.images')} ${site.number(i + 1)}'},
              [ResponsiveImage(im.storageKey, alt: '', sizes: '64px', width: im.width, height: im.height)],
            ),
          ]),
      ]),
    ]);
  }
}
