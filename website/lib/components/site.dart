import 'package:jaspr/jaspr.dart';
import 'package:shared/shared.dart';

/// Ek request er shop ar bhasha (9.8): page ar component `Site.of(context)` diye settings, lekha ar link pay.
class Site extends InheritedComponent {
  /// URL er bhasha (`bn`, `en`).
  final String locale;

  /// Shop er published settings (shop na pele khali, tai getter er default).
  final PublicSettings settings;

  /// UI lekha (default + shop er override).
  final Translator translator;

  /// Site.
  const Site({
    required this.locale,
    required this.settings,
    required this.translator,
    required super.child,
    super.key,
  });

  /// Upore thaka [Site].
  static Site of(BuildContext context) => context.dependOnInheritedComponentOfExactType<Site>()!;

  /// Lekha (9.8).
  String t(String key, [Map<String, Object> args = const {}]) => translator.t(key, args);

  /// Ei bhashar link: `en` e `/en` prefix (10.1).
  String href(String path) => locale == 'en' ? (path == '/' ? '/en' : '/en$path') : path;

  /// Shop e `en` chalu.
  bool get hasEnglish => settings.enabledLocales.contains('en');

  @override
  bool updateShouldNotify(Site oldComponent) => false;
}
