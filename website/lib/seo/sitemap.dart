import 'dart:convert';
import 'dart:io';

import 'package:shared/shared.dart';
import 'package:shelf/shelf.dart';

import '../pages/policy.dart';
import '../services/api.dart';
import '../services/locale.dart';
import '../services/settings_cache.dart';

// robots e block (10.4 item 5); token page `X-Robots-Tag` o pabe (tader milestone e).
const _blocked = ['/search', '/cart', '/checkout', '/account', '/order', '/confirm', '/print'];

/// Shop er `sitemap.xml` (10.4): home, category (tree er shob), product (`/api/products` 100 kore page), policy;
/// [english] hole protiti tar `/en` o.
// ponytail: ek file e shob URL (sitemap er shima 50k); product onek hole sitemap index.
Future<String> sitemapXml(CatalogRepository catalog, {required String origin, required bool english}) async {
  final paths = ['/'];
  void add(List<Category> cs) {
    for (final c in cs) {
      paths.add('/c/${Uri.encodeComponent(c.slug)}');
      add(c.children);
    }
  }

  add(await catalog.categories());
  for (var page = 1; ; page++) {
    final list = await catalog.products(page: page, limit: 100);
    paths.addAll(list.items.map((p) => '/p/${Uri.encodeComponent(p.slug)}'));
    if (list.items.isEmpty || page * 100 >= list.total) break;
  }
  paths.addAll(policyKinds.map((k) => '/policy/$k'));

  final b = StringBuffer('<?xml version="1.0" encoding="UTF-8"?>\n')
    ..writeln('<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">');
  for (final p in paths) {
    for (final l in ['bn', if (english) 'en']) {
      b.writeln(
        '<url><loc>${const HtmlEscape(HtmlEscapeMode.element).convert('$origin${localePath(l, p)}')}</loc></url>',
      );
    }
  }
  b.writeln('</urlset>');
  return b.toString();
}

/// `robots.txt` (10.4): [_blocked] duita bhashay, sitemap er link.
String robotsTxt(String origin) {
  final b = StringBuffer('User-agent: *\n');
  for (final p in _blocked) {
    b.write('Disallow: $p\nDisallow: /en$p\n');
  }
  b.write('\nSitemap: $origin/sitemap.xml\n');
  return b.toString();
}

/// `/sitemap.xml` ar `/robots.txt` page render er age (host er shop er); baki request [inner] e. Shop nai → 404,
/// bondho ba API bhul → 503.
Middleware seoFiles(
  Future<ShopSettings> Function(String host) settingsFor,
  CatalogRepository Function(Request request) catalogFor,
) =>
    (inner) => (request) async {
      final path = '/${request.url.path}';
      if (path != '/sitemap.xml' && path != '/robots.txt') return inner(request);
      // Query diye Cloudflare er cache bypass kore protiti request e puro catalog fetch na.
      if (request.url.hasQuery) return Response.notFound('');
      final origin = originOf(request.headers);
      try {
        final shop = await settingsFor(request.headers['host'] ?? '');
        if (path == '/robots.txt') {
          return Response.ok(robotsTxt(origin), headers: {'content-type': 'text/plain; charset=utf-8'});
        }
        // `noindex` shop e sitemap na (10.4 item 7); robots thake, page er `noindex` crawler ke dekhte hobe.
        if (shop.settings.noindex) return Response.notFound('');
        final english = shop.settings.enabledLocales.contains('en');
        final xml = await sitemapXml(catalogFor(request), origin: origin, english: english);
        return Response.ok(xml, headers: {'content-type': 'application/xml; charset=utf-8'});
      } on ApiError catch (e) {
        if (e.code == 'shop_not_found') return Response.notFound('');
        stderr.writeln('seo file: $e');
        return Response(503);
      } catch (e) {
        stderr.writeln('seo file: $e');
        return Response(503);
      }
    };
