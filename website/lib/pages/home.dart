import 'dart:async';

import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/icon.dart';
import '../components/product_card.dart';
import '../components/responsive_image.dart';
import '../components/site.dart';
import '../components/trust.dart';
import '../services/link.dart';
import 'status.dart';
import '../seo/meta.dart';

/// Home (10.1, 10.5): announcement, banner, category (`#categories`, bottom nav er "Category" eikhane ashe),
/// featured product.
class Home extends AsyncStatelessComponent {
  const Home({super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final s = site.settings;
    final List<Category> categories;
    final List<ProductCard> featured;
    try {
      (categories, featured) = await (site.catalog.categories(), _featured(site.catalog, s.featuredProductSlugs)).wait;
    } on ParallelWaitError<(List<Category>?, List<ProductCard>?), (AsyncError?, AsyncError?)> catch (e) {
      return pageError(context, (e.errors.$1 ?? e.errors.$2)!.error);
    }
    // featured_category_slugs dile shudhu oigulo oi kram e, na hole shob top-level.
    final shown = s.featuredCategorySlugs.isEmpty
        ? categories
        : [
            for (final slug in s.featuredCategorySlugs) ...categories.where((c) => c.slug == slug),
          ];
    final announcement = s.announcement.of(site.locale);
    final tagline = s.tagline.of(site.locale);
    final trust = shopTrust(site);
    return div(classes: 'space-y-8 px-4 py-4', [
      const SeoHead(),
      if (announcement.isNotEmpty)
        // Puro width strip: base p er 70ch na.
        p(classes: 'max-w-none rounded bg-accent-soft px-4 py-2 text-center text-sm', [.text(announcement)]),
      // Page e ekta h1 (10.7 v1.38): banner na thakle dekha text hero (LCP lekha), thakle sr-only, banner LCP.
      if (s.banners.isEmpty)
        div(classes: 'space-y-3 rounded-lg bg-primary-soft px-6 py-10 text-center', [
          h1(classes: 'text-3xl', [.text(s.name.of(site.locale))]),
          // muted shudhu background/surface e 4.5 (9.3), soft e na; base p 70ch, tai mx-auto.
          if (tagline.isNotEmpty) p(classes: 'mx-auto', [.text(tagline)]),
          if (shown.isNotEmpty) a(href: '#categories', classes: 'btn btn-primary', [.text(site.t('category.browse'))]),
        ])
      else
        h1(classes: 'sr-only', [.text(s.name.of(site.locale))]),
      for (final (i, b) in s.banners.indexed) _banner(site, b, lcp: i == 0),
      if (trust.isNotEmpty)
        ul(id: 'trust', classes: 'grid gap-3 sm:grid-cols-2', [
          for (final (icon, text) in trust)
            li(classes: 'flex items-center gap-3 rounded bg-tint px-4 py-3 text-sm', [
              Icon(icon, classes: 'h-6 w-6 shrink-0 text-primary'),
              .text(text),
            ]),
        ]),
      if (shown.isNotEmpty)
        section(id: 'categories', [
          h2(classes: 'section-title', [.text(site.t('home.categories'))]),
          // Tile (plan 4): mobile e side-scroll, boro screen e grid 6; chobi na thakle primary-soft + icon.
          ul(classes: 'flex gap-3 overflow-x-auto pb-2 md:grid md:grid-cols-6 md:overflow-visible', [
            for (final c in shown)
              li(classes: 'w-24 shrink-0 md:w-auto', [
                a(
                  href: site.href('/c/${Uri.encodeComponent(c.slug)}'),
                  classes: 'block space-y-2 text-center text-sm',
                  [
                    if (c.imageKey case final key?)
                      div(classes: 'aspect-square overflow-hidden rounded [&>*]:h-full', [
                        // Naam niche lekha, tai chobi alt khali.
                        ResponsiveImage(key, alt: '', sizes: '(min-width: 768px) 16vw, 6rem'),
                      ])
                    else
                      div(
                        classes: 'flex aspect-square items-center justify-center rounded bg-primary-soft text-primary',
                        [
                          const Icon(Icons.leaf, classes: 'h-1/3 w-1/3'),
                        ],
                      ),
                    span(classes: 'line-clamp-2', [.text(c.name)]),
                  ],
                ),
              ]),
          ]),
        ]),
      if (featured.isNotEmpty)
        section([
          h2(classes: 'section-title', [.text(site.t('home.featured'))]),
          div(classes: 'grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4', [
            for (final (i, p) in featured.indexed) ProductCardView(p, lcp: s.banners.isEmpty && i < 2),
          ]),
        ]),
    ]);
  }

  Component _banner(Site site, Banner b, {required bool lcp}) {
    final image = ResponsiveImage(
      b.imageKey,
      alt: b.alt.of(site.locale),
      sizes: '(min-width: 1280px) 1280px, 100vw',
      width: 1280,
      height: 480,
      lcp: lcp,
    );
    final href = safeLink(b.link, site.locale);
    return div(classes: 'overflow-hidden rounded', [
      href == null ? image : a(href: href, [image]),
    ]);
  }
}

// Featured: protiti slug e detail (10.10 item 10, max 12, ekshathe); na pele (muche gechhe) bad. Card er dam
// API er variant theke bachai (sobcheye kom; hishab na, Rule 1).
Future<List<ProductCard>> _featured(CatalogRepository catalog, List<String> slugs) async {
  final found = await Future.wait([
    for (final slug in slugs.take(12))
      catalog
          .product(slug)
          .then<Product?>((found) => found)
          .catchError(
            (Object _) => null,
            test: (e) => e is ApiError && e.code == 'not_found',
          ),
  ]);
  return [
    for (final prod in found)
      if (prod != null && prod.variants.isNotEmpty) _card(prod),
  ];
}

ProductCard _card(Product p) {
  final cheapest = p.variants.reduce((x, y) => y.price < x.price ? y : x);
  return ProductCard(
    p.id,
    p.slug,
    p.name,
    p.unit,
    p.variableWeight,
    cheapest.price,
    cheapest.compareAtPrice,
    p.variants.any((v) => v.inStock),
    p.rating,
    p.reviewCount,
    p.images.isEmpty ? null : p.images.first.storageKey,
  );
}
