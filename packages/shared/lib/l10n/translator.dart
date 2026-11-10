import 'bn.dart';
import 'en.dart';

/// Developer er default lekha (9.8, v1.12): locale → key → lekha.
const defaultMessages = {'bn': bn, 'en': en};

final _placeholder = RegExp(r'\{(\w+)\}');

/// UI lekha (9.8): admin er override (`GET /api/translations`) ar default file theke.
class Translator {
  /// UI bhasha (`bn` ba `en`).
  final String locale;

  /// Default lekha ([defaultMessages]; test e alada).
  final Map<String, Map<String, String>> defaults;

  /// Shop er override: locale → key → lekha.
  final Map<String, Map<String, String>> overrides;

  /// Kothao na pele (key, locale); app log kore ("missing translation").
  final void Function(String key, String locale)? onMissing;

  /// Translator.
  const Translator(
    this.locale, {
    this.defaults = defaultMessages,
    this.overrides = const {},
    this.onMissing,
  });

  /// `override[locale]` → `file[locale]` → `override[bn]` → `file[bn]` → key; `{name}` er jaygay `args`.
  String t(String key, [Map<String, Object> args = const {}]) {
    final s = _find(key);
    if (s == null) onMissing?.call(key, locale);
    return _fill(s ?? key, args);
  }

  String? _find(String key) =>
      overrides[locale]?[key] ??
      defaults[locale]?[key] ??
      overrides['bn']?[key] ??
      defaults['bn']?[key];

  String _fill(String s, Map<String, Object> args) =>
      s.replaceAllMapped(_placeholder, (m) => args[m[1]]?.toString() ?? m[0]!);

  /// English e `key.one`/`key.other`; Bangla e shob shomoy `.other` (9.8). `.one` kothao na pele
  /// `.other` (Bangla fallback). `{count}` = count.
  String plural(String key, int count, [Map<String, Object> args = const {}]) {
    final a = {...args, 'count': count};
    if (locale == 'en' && count == 1) {
      final one = _find('$key.one');
      if (one != null) return _fill(one, a);
    }
    return t('$key.other', a);
  }
}
