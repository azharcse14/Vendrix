import 'json.dart';

/// List er response (7.1): `{"items","page","limit","total"}`.
class Page<T> {
  /// Ei page er jinish.
  final List<T> items;

  /// Page (1 theke), limit, filter er pore mot.
  final int page, limit, total;

  /// Page.
  const Page(this.items, this.page, this.limit, this.total);

  /// JSON theke; `item` protiti item er fromJson.
  factory Page.fromJson(
    Map<String, dynamic> j,
    T Function(Map<String, dynamic>) item,
  ) => Page(
    objects(j, 'items', item),
    field(j, 'page'),
    field(j, 'limit'),
    field(j, 'total'),
  );

  /// Porer page ache kina.
  bool get hasNext => page * limit < total;
}
