import 'dart:async';

import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/product_listing.dart';
import '../components/site.dart';
import '../services/list_query.dart';
import 'status.dart';

/// Category (`/c/{slug}`, 10.1, 10.5): tree theke naam (na pele 404), [ProductListing].
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
          limit: listLimit,
        ),
      ).wait;
    } on ParallelWaitError<(List<Category>?, Page<ProductCard>?), (AsyncError?, AsyncError?)> catch (e) {
      return pageError(context, (e.errors.$1 ?? e.errors.$2)!.error);
    }
    final category = _find(tree, slug);
    if (category == null) return pageError(context, const ApiError(404, 'not_found'));
    return ProductListing(
      title: category.name,
      action: site.href('/c/${Uri.encodeComponent(slug)}'),
      query: q,
      list: list,
      emptyText: site.t('category.empty'),
    );
  }
}

Category? _find(List<Category> tree, String slug) {
  for (final c in tree) {
    if (c.slug == slug) return c;
    if (_find(c.children, slug) case final found?) return found;
  }
  return null;
}
