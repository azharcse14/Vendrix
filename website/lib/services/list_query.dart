/// `GET /api/products` er `sort` (7.2).
const sorts = ['newest', 'price_asc', 'price_desc', 'popular'];

// Dam er upor shima (taka): poisha int e thake, ar bhul boro shonkhya API te jay na.
const _maxTaka = 10000000;

/// Category/search page er URL query (`?q=&sort=&in_stock=1&min=&max=&page=`). Dam URL e taka (manush lekhe), API te
/// poisha. Bhul value bad, tai API `400` dey na.
class ListQuery {
  /// Search er lekha (trim, API er shima 100 okkhor); khali hole null.
  final String? q;

  /// [sorts] er ekta.
  final String? sort;

  /// Shudhu stock e ache.
  final bool inStock;

  /// URL er taka (form e abar dekhano).
  final int? minTaka, maxTaka;

  /// 1 theke.
  final int page;

  /// ListQuery.
  const ListQuery({this.q, this.sort, this.inStock = false, this.minTaka, this.maxTaka, this.page = 1});

  /// URL query theke.
  factory ListQuery.parse(Map<String, String> q) {
    int? taka(String? v) {
      final n = int.tryParse(v ?? '');
      return n != null && n >= 0 && n <= _maxTaka ? n : null;
    }

    final page = int.tryParse(q['page'] ?? '');
    final text = String.fromCharCodes((q['q'] ?? '').trim().runes.take(100)).trim();
    return ListQuery(
      q: text.isEmpty ? null : text,
      sort: sorts.contains(q['sort']) ? q['sort'] : null,
      inStock: q['in_stock'] == '1',
      minTaka: taka(q['min']),
      maxTaka: taka(q['max']),
      page: page != null && page >= 1 && page <= 10000 ? page : 1,
    );
  }

  /// API er `min_price` (poisha).
  int? get minPoisha => minTaka == null ? null : minTaka! * 100;

  /// API er `max_price` (poisha).
  int? get maxPoisha => maxTaka == null ? null : maxTaka! * 100;

  /// Filter thik rekhe [page] er query (1 e `page` na).
  Map<String, String> withPage(int page) => {
    'q': ?q,
    'sort': ?sort,
    if (inStock) 'in_stock': '1',
    'min': ?minTaka?.toString(),
    'max': ?maxTaka?.toString(),
    if (page > 1) 'page': '$page',
  };
}
