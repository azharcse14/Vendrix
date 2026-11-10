/// JSON field type mile na hole [FormatException] (server er shape bodlale chupchap bhul data na).
T field<T>(Map<String, dynamic> j, String key) {
  final v = j[key];
  if (v is T) return v;
  throw FormatException('$key: $T chai, ${v.runtimeType} peyechi');
}

/// Shonkhya (JSON e `0` int ashe) → double.
double number(Map<String, dynamic> j, String key) =>
    field<num>(j, key).toDouble();

/// Object er list.
List<T> objects<T>(
  Map<String, dynamic> j,
  String key,
  T Function(Map<String, dynamic>) f,
) => [
  for (final e in field<List<dynamic>?>(j, key) ?? const [])
    f(e as Map<String, dynamic>),
];
