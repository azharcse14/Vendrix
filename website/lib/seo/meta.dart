import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';

import '../components/site.dart';
import '../services/locale.dart';
import 'json_ld.dart';

/// Page er `<head>` SEO (10.4): `<title>`, `meta description`, OG (`property` attribute), `canonical`, `hreflang`
/// (bn, en (chalu thakle), x-default), `robots noindex` (page ba shop er `seo.noindex`), JSON-LD.
class SeoHead extends StatelessComponent {
  /// Page er shironam; `null` hole home (shop er `seo.title`, na hole naam). Baki page e "x | shop".
  final String? title;

  /// `null` hole shop er `seo.description`.
  final String? description;

  /// OG chobi er key; `null` hole shop er `og_image_key`, na hole logo.
  final String? imageKey;

  /// `og:type`.
  final String type;

  /// `product:price:amount` (poisha).
  final int? price;

  /// JSON-LD ([jsonLd] e escape hoy).
  final Map<String, Object?>? data;

  /// Ei page `noindex` (search).
  final bool noindex;

  /// SeoHead.
  const SeoHead({
    this.title,
    this.description,
    this.imageKey,
    this.type = 'website',
    this.price,
    this.data,
    this.noindex = false,
    super.key,
  });

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final s = site.settings;
    final shopName = s.name.of(site.locale);
    final seoTitle = s.seoTitle.of(site.locale);
    final full = switch (title) {
      null => seoTitle.isEmpty ? shopName : seoTitle,
      final t when shopName.isEmpty => t,
      final t => '$t | $shopName',
    };
    final desc = _short(description ?? s.seoDescription.of(site.locale));
    final path = localeOf(Uri.parse(context.url).path).path;
    String url(String locale) => '${site.origin}${localePath(locale, path)}';
    final key = imageKey ?? s.ogImageKey ?? s.logoKey;
    final image = key == null ? null : site.images.url(key, 1280);
    Component og(String property, String content) => meta(attributes: {'property': property, 'content': content});

    return Document.head(
      title: full,
      meta: {
        if (desc.isNotEmpty) 'description': desc,
        if (noindex || s.noindex) 'robots': 'noindex',
      },
      children: [
        link(rel: 'canonical', href: url(site.locale)),
        for (final (lang, href) in [
          ('bn', url('bn')),
          if (site.hasEnglish) ('en', url('en')),
          ('x-default', url('bn')),
        ])
          link(rel: 'alternate', href: href, attributes: {'hreflang': lang}),
        og('og:title', title ?? full),
        if (desc.isNotEmpty) og('og:description', desc),
        og('og:type', type),
        og('og:url', url(site.locale)),
        og('og:site_name', shopName),
        if (image != null) og('og:image', image),
        if (price case final p?) ...[og('product:price:amount', amount(p)), og('product:price:currency', 'BDT')],
        if (data case final d?)
          // `content` raw boshe; jsonLd e `<` escape kora, tai `</script>` ashte pare na.
          script(attributes: {'type': 'application/ld+json'}, content: jsonLd(d)),
      ],
    );
  }
}

// Description 160 okkhor, ek line.
String _short(String s) {
  final one = s.replaceAll(RegExp(r'\s+'), ' ').trim();
  final r = one.runes;
  return r.length <= 160 ? one : '${String.fromCharCodes(r.take(159))}…';
}
