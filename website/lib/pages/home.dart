import 'dart:async';

import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/product_card.dart';
import '../components/responsive_image.dart';
import '../components/site.dart';
import '../services/link.dart';
import 'status.dart';

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
    return div(classes: 'space-y-8 px-4 py-4', [
      if (announcement.isNotEmpty) p(classes: 'rounded bg-tint px-4 py-2 text-center text-sm', [.text(announcement)]),
      for (final (i, b) in s.banners.indexed) _banner(site, b, lcp: i == 0),
      if (shown.isNotEmpty)
        section(id: 'categories', [
          h2(classes: 'mb-3', [.text(site.t('home.categories'))]),
          // Mobile e side-scroll, boro screen e wrap (10.5).
          ul(classes: 'flex gap-2 overflow-x-auto pb-2 md:flex-wrap md:overflow-visible', [
            for (final c in shown)
              li([
                a(
                  href: site.href('/c/${Uri.encodeComponent(c.slug)}'),
                  classes: 'flex min-h-11 items-center whitespace-nowrap rounded border border-line bg-surface px-4',
                  [.text(c.name)],
                ),
              ]),
          ]),
        ]),
      if (featured.isNotEmpty)
        section([
          h2(classes: 'mb-3', [.text(site.t('home.featured'))]),
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
