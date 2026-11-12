import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:shared/shared.dart';

import '../services/list_query.dart';
import 'filter_sheet.dart';
import 'product_card.dart';
import 'site.dart';

/// Ek page e koyta product (2/3/4 column e mile).
const listLimit = 24;

/// Category ar search er list (10.1, 10.5): shironam, koyta, product grid, sort/filter, paging (`?page=`).
/// Mobile e filter `<dialog>` e ([FilterSheet]), desktop e bame sidebar.
class ProductListing extends StatelessComponent {
  /// `h1`.
  final String title;

  /// Page er link (bhasha shoho, query chhara).
  final String action;

  /// URL query.
  final ListQuery query;

  /// API er page.
  final Page<ProductCard> list;

  /// List khali hole lekha.
  final String emptyText;

  /// ProductListing.
  const ProductListing({
    required this.title,
    required this.action,
    required this.query,
    required this.list,
    required this.emptyText,
    super.key,
  });

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final q = query;
    final form = FilterForm(
      action: action,
      q: q.q,
      sort: q.sort,
      inStock: q.inStock,
      minTaka: q.minTaka,
      maxTaka: q.maxTaka,
    );
    final pages = (list.total + listLimit - 1) ~/ listLimit;
    String pageHref(int n) {
      final params = q.withPage(n);
      return params.isEmpty ? action : '$action?${Uri(queryParameters: params).query}';
    }

    Component pageLink(int n, String rel, String key) => a(
      href: pageHref(n),
      classes: 'flex min-h-11 items-center px-4 underline',
      attributes: {'rel': rel},
      [.text(site.t(key))],
    );

    return div(classes: 'px-4 py-4', [
      h1(classes: 'mb-1', [.text(title)]),
      p(classes: 'mb-4 text-sm text-muted', [
        .text(site.translator.plural('category.count', list.total, {'count': site.number(list.total)})),
      ]),
      div(classes: 'lg:grid lg:grid-cols-[16rem_1fr] lg:gap-8', [
        aside(classes: 'hidden lg:block', [form]),
        div([
          // Mobile e sticky (10.5).
          div(classes: 'sticky top-0 z-10 -mx-4 mb-3 bg-page px-4 py-2 lg:hidden', [FilterSheet(form)]),
          if (list.items.isEmpty)
            p(classes: 'py-12 text-center text-muted', [.text(emptyText)])
          else
            div(classes: 'grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-3 xl:grid-cols-4', [
              for (final (i, item) in list.items.indexed) ProductCardView(item, lcp: q.page == 1 && i < 2),
            ]),
          if (pages > 1)
            nav(
              classes: 'mt-6 flex items-center justify-between gap-2',
              attributes: {'aria-label': site.t('page.nav')},
              [
                if (q.page > 1) pageLink(q.page - 1, 'prev', 'page.prev') else span([]),
                span(classes: 'text-sm text-muted', [
                  .text(site.t('page.of', {'page': site.number(q.page), 'pages': site.number(pages)})),
                ]),
                if (q.page < pages) pageLink(q.page + 1, 'next', 'page.next') else span([]),
              ],
            ),
        ]),
      ]),
    ]);
  }
}
