import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/product_listing.dart';
import '../components/site.dart';
import '../services/list_query.dart';
import 'status.dart';
import '../seo/meta.dart';

/// Search (`/search?q=`, 10.1): `products(q:)` diye [ProductListing]; `noindex` (10.4). `q` 2 okkhorer kom hole
/// API call na (API oikhane search kore na, 6.4), shudhu lekha.
class SearchPage extends AsyncStatelessComponent {
  /// URL query.
  final Map<String, String> query;

  /// SearchPage.
  const SearchPage(this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final q = ListQuery.parse(query);
    final text = q.q;
    if (text == null || text.runes.length < 2) {
      return div(classes: 'px-4 py-12 text-center text-muted', [
        SeoHead(title: site.t('search.submit'), noindex: true),
        p([.text(site.t('search.prompt'))]),
      ]);
    }
    final Page<ProductCard> list;
    try {
      list = await site.catalog.products(
        q: text,
        sort: q.sort,
        inStock: q.inStock ? true : null,
        minPrice: q.minPoisha,
        maxPrice: q.maxPoisha,
        page: q.page,
        limit: listLimit,
      );
    } catch (e) {
      return pageError(context, e);
    }
    return ProductListing(
      title: site.t('search.title', {'q': text}),
      action: site.href('/search'),
      query: q,
      list: list,
      emptyText: site.t('search.empty'),
      noindex: true,
    );
  }
}
