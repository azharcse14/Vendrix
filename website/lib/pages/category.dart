import 'dart:async';

import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/filter_sheet.dart';
import '../components/product_card.dart';
import '../components/site.dart';
import '../services/list_query.dart';
import 'status.dart';

// Ek page e koyta product (2/3/4 column e mile).
const _limit = 24;

/// Category (`/c/{slug}`, 10.1, 10.5): naam, product grid, sort/filter (`?sort=&in_stock=1&min=&max=`), paging
/// (`?page=`). Mobile e filter `<dialog>` e ([FilterSheet]), desktop e bame sidebar.
class CategoryPage extends AsyncStatelessComponent {
  /// URL er slug.
  final String slug;

  /// URL query.
  final Map<String, String> query;

  /// CategoryPage.
  const CategoryPage(this.slug, this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final q = ListQuery.parse(query);
    final List<Category> tree;
    final Page<ProductCard> list;
    try {
      (tree, list) = await (
        site.catalog.categories(),
        site.catalog.products(
          category: slug,
          sort: q.sort,
          inStock: q.inStock ? true : null,
          minPrice: q.minPoisha,
          maxPrice: q.maxPoisha,
          page: q.page,
          limit: _limit,
        ),
      ).wait;
    } on ParallelWaitError<(List<Category>?, Page<ProductCard>?), (AsyncError?, AsyncError?)> catch (e) {
      return pageError(context, (e.errors.$1 ?? e.errors.$2)!.error);
    }
    final category = _find(tree, slug);
    if (category == null) return pageError(context, const ApiError(404, 'not_found'));

    final action = site.href('/c/${Uri.encodeComponent(slug)}');
    final form = FilterForm(
      action: action,
      sort: q.sort,
      inStock: q.inStock,
      minTaka: q.minTaka,
      maxTaka: q.maxTaka,
    );
    final pages = (list.total + _limit - 1) ~/ _limit;
    String pageHref(int n) {
      final params = q.withPage(n);
      return params.isEmpty ? action : '$action?${Uri(queryParameters: params).query}';
    }

    return div(classes: 'px-4 py-4', [
      h1(classes: 'mb-1', [.text(category.name)]),
      p(classes: 'mb-4 text-sm text-muted', [
        .text(site.translator.plural('category.count', list.total, {'count': site.number(list.total)})),
      ]),
      div(classes: 'lg:grid lg:grid-cols-[16rem_1fr] lg:gap-8', [
        aside(classes: 'hidden lg:block', [
          form,
        ]),
        div([
          // Mobile e sticky (10.5).
          div(classes: 'sticky top-0 z-10 -mx-4 mb-3 bg-page px-4 py-2 lg:hidden', [
            FilterSheet(form),
          ]),
          if (list.items.isEmpty)
            p(classes: 'py-12 text-center text-muted', [.text(site.t('category.empty'))])
          else
            div(classes: 'grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-3 xl:grid-cols-4', [
              for (final (i, item) in list.items.indexed) ProductCardView(item, lcp: q.page == 1 && i < 2),
            ]),
          if (pages > 1)
            nav(
              classes: 'mt-6 flex items-center justify-between gap-2',
              attributes: {'aria-label': site.t('page.nav')},
              [
                if (q.page > 1)
                  a(
                    href: pageHref(q.page - 1),
                    classes: 'flex min-h-11 items-center px-4 underline',
                    attributes: {'rel': 'prev'},
                    [
                      .text(site.t('page.prev')),
                    ],
                  )
                else
                  span([]),
                span(classes: 'text-sm text-muted', [
                  .text(site.t('page.of', {'page': site.number(q.page), 'pages': site.number(pages)})),
                ]),
                if (q.page < pages)
                  a(
                    href: pageHref(q.page + 1),
                    classes: 'flex min-h-11 items-center px-4 underline',
                    attributes: {'rel': 'next'},
                    [
                      .text(site.t('page.next')),
                    ],
                  )
                else
                  span([]),
              ],
            ),
        ]),
      ]),
    ]);
  }
}

Category? _find(List<Category> tree, String slug) {
  for (final c in tree) {
    if (c.slug == slug) return c;
    if (_find(c.children, slug) case final found?) return found;
  }
  return null;
}
