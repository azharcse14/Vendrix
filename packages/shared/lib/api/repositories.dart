import '../models/catalog.dart';
import '../models/page.dart';
import '../models/settings.dart';
import 'api_client.dart';

/// Public catalog (7.2): category, product list (filter, search, paging), detail, shop er policy.
class CatalogRepository {
  /// Client.
  final ApiClient api;

  /// CatalogRepository.
  const CatalogRepository(this.api);

  /// `GET /api/categories` (tree).
  Future<List<Category>> categories() async => [
    for (final c in await api.get('/api/categories') as List<dynamic>)
      Category.fromJson(c as Map<String, dynamic>),
  ];

  /// `GET /api/products`; dam poisha. Na deya filter query te jay na.
  Future<Page<ProductCard>> products({
    String? q,
    String? category,
    String? vendor,
    int? minPrice,
    int? maxPrice,
    bool? inStock,
    String? sort,
    int? page,
    int? limit,
  }) async {
    final query = {
      'q': q,
      'category': category,
      'vendor': vendor,
      'min_price': minPrice?.toString(),
      'max_price': maxPrice?.toString(),
      'in_stock': inStock?.toString(),
      'sort': sort,
      'page': page?.toString(),
      'limit': limit?.toString(),
    }..removeWhere((_, v) => v == null);
    final j =
        await api.get('/api/products', query: query.cast<String, String>())
            as Map<String, dynamic>;
    return Page.fromJson(j, ProductCard.fromJson);
  }

  /// `GET /api/products/{slug}`; nai hole `ApiError` `not_found`.
  Future<Product> product(String slug) async => Product.fromJson(
    await api.get('/api/products/${Uri.encodeComponent(slug)}')
        as Map<String, dynamic>,
  );

  /// `GET /api/policies/{kind}` er `body` (plain text; locale er, na hole Bangla, v1.13); ochena kind `not_found`.
  Future<String> policy(String kind) async =>
      (await api.get('/api/policies/${Uri.encodeComponent(kind)}')
              as Map<String, dynamic>)['body']
          as String;
}

/// Shop er public settings ar translation override (7.2, 9.8).
class SettingsRepository {
  /// Client.
  final ApiClient api;

  /// SettingsRepository.
  const SettingsRepository(this.api);

  /// `GET /api/settings`.
  Future<PublicSettings> settings() async => PublicSettings.fromJson(
    await api.get('/api/settings') as Map<String, dynamic>,
  );

  /// `GET /api/settings/version` (cache check).
  Future<int> version() async =>
      (await api.get('/api/settings/version')
              as Map<String, dynamic>)['version']
          as int;

  /// `GET /api/translations?locale=` → `Translator` er `overrides[locale]`.
  Future<Map<String, String>> translations(String locale) async =>
      (await api.get('/api/translations', query: {'locale': locale})
              as Map<String, dynamic>)
          .cast<String, String>();
}
