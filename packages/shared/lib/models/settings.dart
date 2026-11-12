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

  LocalizedText _text(String block, String key) => LocalizedText.fromJson(
    _block(block)[key] is Map<String, dynamic>
        ? _block(block)[key] as Map<String, dynamic>
        : const {},
  );

  /// `homepage.announcement`.
  LocalizedText get announcement => _text('homepage', 'announcement');

  /// `seo.title` (home er `<title>`, 10.4).
  LocalizedText get seoTitle => _text('seo', 'title');

  /// `seo.description`.
  LocalizedText get seoDescription => _text('seo', 'description');

  /// `seo.og_image_key`.
  String? get ogImageKey => _str('seo', 'og_image_key');

  /// `seo.noindex`: puro site `noindex` (limited launch, 14.4); na thakle false.
  bool get noindex => _block('seo')['noindex'] == true;

  List<String> _strings(String block, String key) => [
    for (final v in _block(block)[key] as List<dynamic>? ?? const [])
      if (v is String) v,
  ];

  /// `homepage.banners`; `image_key` string na hole item bad. `link` 9.2 e validate, tobu render e abar check (12.2).
  List<Banner> get banners => [
    for (final b in _block('homepage')['banners'] as List<dynamic>? ?? const [])
      if (b is Map<String, dynamic> && b['image_key'] is String)
        Banner(
          b['image_key'] as String,
          LocalizedText.fromJson(
            b['alt'] is Map<String, dynamic>
                ? b['alt'] as Map<String, dynamic>
                : const {},
          ),
          b['link'] is String ? b['link'] as String : null,
        ),
  ];

  /// `homepage.featured_category_slugs`.
  List<String> get featuredCategorySlugs =>
      _strings('homepage', 'featured_category_slugs');

  /// `homepage.featured_product_slugs`.
  List<String> get featuredProductSlugs =>
      _strings('homepage', 'featured_product_slugs');
}

/// Home er banner (9.2).
class Banner {
  /// Chobi (storage key).
  final String imageKey;

  /// `alt`.
  final LocalizedText alt;

  /// `/...` ba `https://...`.
  final String? link;

  /// Banner.
  const Banner(this.imageKey, this.alt, this.link);
}
