import 'localized_text.dart';

/// `GET /api/settings` er public settings (7.2, 9.2). Typed getter shudhu dorkari gulo; baki [raw].
class PublicSettings {
  /// Puro JSON.
  final Map<String, dynamic> raw;

  /// PublicSettings.
  const PublicSettings(this.raw);

  /// JSON theke.
  factory PublicSettings.fromJson(Map<String, dynamic> j) => PublicSettings(j);

  Map<String, dynamic> _block(String k) => raw[k] is Map<String, dynamic>
      ? raw[k] as Map<String, dynamic>
      : const {};

  /// `branding.name`.
  LocalizedText get name => LocalizedText.fromJson(
    _block('branding')['name'] is Map<String, dynamic>
        ? _block('branding')['name'] as Map<String, dynamic>
        : const {},
  );

  /// `theme.colors` (5 rong, 9.4); `deriveTokens` e dao.
  Map<String, String> get colors =>
      (_block('theme')['colors'] as Map<String, dynamic>? ?? const {})
          .cast<String, String>();

  /// `locales.default` (na thakle `bn`).
  String get defaultLocale => _block('locales')['default'] as String? ?? 'bn';

  /// `locales.enabled` (na thakle `['bn']`).
  List<String> get enabledLocales =>
      (_block('locales')['enabled'] as List<dynamic>? ?? const ['bn'])
          .cast<String>();

  /// `locales.bangla_digits` (na thakle on).
  bool get banglaDigits => _block('locales')['bangla_digits'] as bool? ?? true;

  /// `checkout.payment_methods`.
  List<String> get paymentMethods =>
      (_block('checkout')['payment_methods'] as List<dynamic>? ?? const [])
          .cast<String>();

  String? _str(String block, String key) =>
      _block(block)[key] is String ? _block(block)[key] as String : null;

  Map<String, dynamic> get _fonts =>
      _block('theme')['fonts'] is Map<String, dynamic>
      ? _block('theme')['fonts'] as Map<String, dynamic>
      : const {};

  /// `theme.fonts.heading` (na thakle Hind Siliguri).
  String get headingFont => _fonts['heading'] is String
      ? _fonts['heading'] as String
      : 'Hind Siliguri';

  /// `theme.fonts.body` (na thakle Hind Siliguri).
  String get bodyFont =>
      _fonts['body'] is String ? _fonts['body'] as String : 'Hind Siliguri';

  /// `theme.radius` px (na thakle 8).
  int get radius =>
      _block('theme')['radius'] is int ? _block('theme')['radius'] as int : 8;

  /// `branding.logo_key`.
  String? get logoKey => _str('branding', 'logo_key');

  /// `branding.phone`.
  String? get phone => _str('branding', 'phone');

  /// `branding.email`.
  String? get email => _str('branding', 'email');

  /// `branding.dbid_number` (footer, 11.x compliance).
  String? get dbidNumber => _str('branding', 'dbid_number');

  /// `branding.bfsa_number`.
  String? get bfsaNumber => _str('branding', 'bfsa_number');

  /// `homepage.announcement`.
  LocalizedText get announcement => LocalizedText.fromJson(
    _block('homepage')['announcement'] is Map<String, dynamic>
        ? _block('homepage')['announcement'] as Map<String, dynamic>
        : const {},
  );
}
