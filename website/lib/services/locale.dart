/// [path] (`/en` chhara) er [locale] er link: `en` e `/en` prefix (10.1).
String localePath(String locale, String path) => locale == 'en' ? (path == '/' ? '/en' : '/en$path') : path;

/// URL theke bhasha (9.8, 10.3): `/en` ba `/en/...` → `en`, baki `bn`; browser er `Accept-Language` dekha hoy na
/// (cache er jonno URL-i bhasha). [path] e `/en` chhara baki route.
({String locale, String path}) localeOf(String path) {
  if (path == '/en' || path == '/en/') return (locale: 'en', path: '/');
  if (path.startsWith('/en/')) return (locale: 'en', path: path.substring(3));
  return (locale: 'bn', path: path);
}
