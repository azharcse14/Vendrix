import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../app.dart';
import '../components/site.dart';
import '../pages/status.dart';
import 'api.dart';
import 'config.dart';
import 'image.dart';
import 'locale.dart';
import 'settings_cache.dart';
import 'theme.dart';

/// Ek request er page: status, extra header ar [Site] (child = page).
typedef ResolvedPage = ({int status, Map<String, String> headers, Site site, String? css});

/// Page thik kore (10.10 item 10): shop nai → 404; bondho → 503 + `Retry-After: 3600`; API down ba onno bhul →
/// 503 error page; `locales.enabled` e `en` na thakle `/en/*` 404. Non-2xx `no-store` (`cacheControl`).
Future<ResolvedPage> resolvePage(
  String host,
  String path,
  Future<ShopSettings> Function(String host) settingsFor, {
  required String origin,
  required CatalogRepository Function(String locale) catalogFor,
  required ImageUrls images,
  required void Function(Object error) log,
}) async {
  final locale = localeOf(path).locale;
  ResolvedPage status(int code, StatusKind kind, [Map<String, String> headers = const {}]) => (
    status: code,
    headers: headers,
    // Shop chhara: default lekha, theme na (browser er default rong).
    site: Site(
      origin: origin,
      locale: locale,
      settings: const PublicSettings({}),
      translator: Translator(locale),
      catalog: catalogFor(locale),
      images: images,
      child: StatusPage(kind),
    ),
    css: null,
  );

  final ShopSettings shop;
  try {
    shop = await settingsFor(host);
  } on ApiError catch (e) {
    if (e.code == 'shop_not_found') return status(404, StatusKind.notFound);
    if (e.code == 'shop_suspended') return status(503, StatusKind.closed, {'retry-after': '3600'});
    log(e);
    return status(503, StatusKind.error);
  } catch (e) {
    log(e);
    return status(503, StatusKind.error);
  }

  // Shop e `en` bondho hole `/en/*` 404, lekha Bangla e.
  final missing = locale == 'en' && !shop.settings.enabledLocales.contains('en');
  final lang = missing ? 'bn' : locale;
  String? css;
  try {
    css = themeCss(shop.settings);
  } catch (e) {
    // Settings 9.2 e validate hoy, tai hoy na; hole page thake, default rong.
    log(e);
  }
  return (
    status: missing ? 404 : 200,
    headers: const <String, String>{},
    site: Site(
      origin: origin,
      locale: lang,
      settings: shop.settings,
      translator: Translator(
        lang,
        overrides: shop.translations,
        onMissing: (key, l) => log('missing translation: $key ($l)'),
      ),
      catalog: catalogFor(lang),
      images: images,
      child: missing ? const StatusPage(StatusKind.notFound) : const App(),
    ),
    css: css,
  );
}

/// `serveApp` er handler: [resolvePage] er page `Document` e render kore status ar header boshay.
Future<Response> renderPage(
  Request request,
  RenderFunction render,
  Future<ShopSettings> Function(String host) settingsFor, {
  required void Function(Object error) log,
}) async {
  final page = await resolvePage(
    request.headers['host'] ?? '',
    '/${request.url.path}',
    settingsFor,
    origin: originOf(request.headers),
    catalogFor: (locale) => CatalogRepository(apiFor(request.headers, locale, config, httpClient)),
    images: ImageUrls(config.imageBaseUrl, resize: config.imageResize),
    log: log,
  );
  final res = await render(
    Document(
      lang: page.site.locale,
      viewport: 'width=device-width, initial-scale=1, viewport-fit=cover',
      head: [
        link(rel: 'stylesheet', href: '/styles.css'),
        // Raw: CSS shudhu validate kora rong/font/radius theke (`cssVariables`), `</style>` ashte pare na.
        if (page.css != null) Component.element(tag: 'style', children: [RawText(page.css!)]),
      ],
      body: page.site,
    ),
  );
  if (page.status == 200 && page.headers.isEmpty) return res;
  // Router er 404 (`NotFound`) render e status dey; tai shudhu 200 hole amader status boshe.
  return Response(
    res.statusCode == 200 ? page.status : res.statusCode,
    body: res.read(),
    headers: {...res.headersAll, ...page.headers},
  );
}
