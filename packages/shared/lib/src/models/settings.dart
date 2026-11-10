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
}
