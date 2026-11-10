/// Content er lekha `{"bn","en"}` (7.1 admin API, 9.8). Public API age-i resolve kore dey.
class LocalizedText {
  /// locale → lekha.
  final Map<String, String> values;

  /// LocalizedText.
  const LocalizedText(this.values);

  /// JSON object theke (shudhu string value).
  factory LocalizedText.fromJson(Map<String, dynamic> json) => LocalizedText({
    for (final e in json.entries)
      if (e.value is String) e.key: e.value as String,
  });

  /// `value[locale]`, khali ba nai hole `bn` (9.8).
  String of(String locale) {
    final s = values[locale];
    return s == null || s.isEmpty ? values['bn'] ?? '' : s;
  }

  /// JSON.
  Map<String, String> toJson() => values;
}
