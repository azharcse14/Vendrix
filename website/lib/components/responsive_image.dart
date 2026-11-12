import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import 'site.dart';

/// Chobi (10.5 niyom 7): `srcset` + `sizes`, `width`/`height` (CLS na), LCP chobi te `fetchpriority="high"`, baki
/// `loading="lazy"`. Key na thakle ba bhul hole `tint` placeholder (10.10 item 3).
class ResponsiveImage extends StatelessComponent {
  /// `storage_key`.
  final String? storageKey;

  /// `alt` (product er naam).
  final String alt;

  /// `sizes`.
  final String sizes;

  /// Pixel (aspect ratio er jonno).
  final int width, height;

  /// Prothom screen er boro chobi.
  final bool lcp;

  /// ResponsiveImage.
  const ResponsiveImage(
    this.storageKey, {
    required this.alt,
    required this.sizes,
    this.width = 640,
    this.height = 640,
    this.lcp = false,
    super.key,
  });

  @override
  Component build(BuildContext context) {
    final images = Site.of(context).images;
    final key = storageKey;
    final src = key == null ? null : images.url(key, 640);
    if (key == null || src == null) {
      return div(
        classes: 'aspect-square w-full bg-tint',
        attributes: {'role': 'img', 'aria-label': alt},
        [],
      );
    }
    return img(
      src: src,
      alt: alt,
      width: width,
      height: height,
      loading: lcp ? null : MediaLoading.lazy,
      classes: 'h-auto w-full bg-tint object-cover',
      attributes: {
        'srcset': ?images.srcset(key),
        'sizes': sizes,
        'decoding': 'async',
        if (lcp) 'fetchpriority': 'high',
      },
    );
  }
}
