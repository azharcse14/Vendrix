import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:http/http.dart' as http;
import 'package:shared/shared.dart';
import 'package:shelf/shelf.dart';

import 'api.dart';
import 'config.dart';
import 'locale.dart';

/// BFF er ekta route (10.9): [methods] ar [path] mille Go API er [api] path e forward.
class BffRoute {
  /// HTTP method.
  final Set<String> methods;

  /// `/_/...` path (puro mil).
  final RegExp path;

  /// Mil theke Go API er path.
  final String Function(Match) api;

  /// Go API er method; null = request er method (JS chhara form shudhu POST pare).
  final String? apiMethod;

  /// Go er uttor, request er cookie ar notun token er [ApiClient] theke browser er body ar `Set-Cookie` (auth 10.11
  /// item 4, cart 10.12); null = Go er uttor bhanga.
  final FutureOr<(List<int>, List<String>)?> Function(
    http.Response res,
    Map<String, String> cookies,
    ApiClient Function(String token) api,
  )?
  session;

  /// JS chhara form er ei field gulo JSON e number (Go er `int`); parse na hole lekha thake, Go `400` dey.
  final Set<String> ints;

  /// JS chhara form 2xx e ekhane `303` (bhasha prefix shoho); null = form er page e.
  final String? formDone;

  /// BffRoute.
  const BffRoute(
    this.methods,
    this.path,
    this.api, {
    this.session,
    this.apiMethod,
    this.ints = const {},
    this.formDone,
  });
}

// Go API er `httpx.MaxBody` (12.2): er beshi body memory te pora hoy na.
const _maxBody = 1 << 20;

String _cartApi(Match m) => '/api/cart/items/${m[1]}';

/// Asol route (10.9): M4 e auth ar account, M5 e cart ar wishlist.
final bffRoutes = <BffRoute>[
  BffRoute({'POST'}, RegExp(r'^/_/auth/otp/send$'), (_) => '/api/auth/otp/send'),
  BffRoute({'POST'}, RegExp(r'^/_/auth/otp/verify$'), (_) => '/api/auth/otp/verify', session: _login),
  BffRoute({'POST'}, RegExp(r'^/_/auth/logout$'), (_) => '/api/auth/logout', session: _logout),
  // POST: account page er JS chhara form (10.2).
  BffRoute({'PATCH', 'POST'}, RegExp(r'^/_/account$'), (_) => '/api/me', apiMethod: 'PATCH'),
  BffRoute(
    {'POST'},
    RegExp(r'^/_/cart/items$'),
    (_) => '/api/cart/items',
    session: _cart,
    ints: {'variant_id', 'quantity'},
    formDone: '/cart',
  ),
  BffRoute({'DELETE'}, RegExp(r'^/_/cart/items/(\d+)$'), _cartApi, session: _cart, formDone: '/cart'),
  // POST: cart page er JS chhara qty form (10.12 item 4).
  BffRoute(
    {'PATCH', 'POST'},
    RegExp(r'^/_/cart/items/(\d+)$'),
    _cartApi,
    apiMethod: 'PATCH',
    session: _cart,
    ints: {'quantity'},
    formDone: '/cart',
  ),
  BffRoute(
    {'POST'},
    RegExp(r'^/_/cart/items/(\d+)/delete$'),
    _cartApi,
    apiMethod: 'DELETE',
    session: _cart,
    formDone: '/cart',
  ),
  // Shudhu login e (guest er `wl` cookie JS nijei lekhe, 10.12 item 3).
  BffRoute({'POST', 'DELETE'}, RegExp(r'^/_/wishlist/(\d+)$'), (m) => '/api/wishlist/${m[1]}'),
];

// Go er `rand.Text()` base32; onno okkhor (`;`, space) cookie header bhangte pare.
final _tokenChars = RegExp(r'^[A-Za-z0-9_-]+$');

// Go er cart key (`rand.Text()`, 8.4); onno kichu Go "nai" dhore, tai forward ba abar set kori na.
final _cartKey = RegExp(r'^[A-Z2-7]{26}$');

const _month = 30 * 24 * 3600;

String _sid(String token, int maxAge) => '__Host-sid=$token; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=$maxAge';

String _cartCookie(String key, int maxAge) =>
    '__Host-cart=$key; HttpOnly; Secure; SameSite=Lax; Path=/; Max-Age=$maxAge';

/// Cookie er `__Host-sid` token er okkhor e hole seta, na hole null (khali ba bhul okkhor Bearer e gele header
/// bhange ba bhul `401`; tokhon guest, 10.12 item 8).
String? validSid(Map<String, String> cookies) {
  final s = cookies['__Host-sid'];
  return s != null && _tokenChars.hasMatch(s) ? s : null;
}

/// Cookie er `__Host-cart` Go er format e hole seta, na hole null (Go bhul key "nai" dhore, tai pathai na).
String? validCartKey(Map<String, String> cookies) {
  final k = cookies['__Host-cart'];
  return k != null && _cartKey.hasMatch(k) ? k : null;
}

// verify: token shudhu HttpOnly cookie te, body theke muche (JS kokhono token dekhe na, 10.11 item 4). Go cart
// merge koreche, tai `__Host-cart` muche; `wl` merge (10.12 item 2, 3).
Future<(List<int>, List<String>)?> _login(
  http.Response res,
  Map<String, String> cookies,
  ApiClient Function(String token) api,
) async {
  if (res.statusCode != 200) return (res.bodyBytes, const <String>[]);
  try {
    if (jsonDecode(res.body) case final Map<String, dynamic> m) {
      if (m.remove('token') case final String t when _tokenChars.hasMatch(t)) {
        return (
          utf8.encode(jsonEncode(m)),
          [
            _sid(t, _month),
            if (cookies.containsKey('__Host-cart')) _cartCookie('', 0),
            if (cookies['wl'] case final wl? when await _mergeWishlist(wl, api(t)))
              'wl=; Path=/; Secure; SameSite=Lax; Max-Age=0',
          ],
        );
      }
    }
  } on FormatException {
    // niche 502
  }
  return null;
}

// Fail hole login fail na; `wl` thake, porer login e abar chesta (10.12 item 3).
Future<bool> _mergeWishlist(String wl, ApiClient api) async {
  final ids = [
    for (final s in wl.split(','))
      if (int.tryParse(s) case final id? when id > 0) id,
  ].take(50).toList();
  if (ids.isEmpty) return true;
  try {
    await WishlistRepository(api).merge(ids);
    return true;
  } catch (e) {
    stderr.writeln('wishlist merge: $e');
    return false;
  }
}

// logout: Go revoke na korte parleo (401, session expire) browser er cookie muche.
(List<int>, List<String>) _logout(http.Response res, Map<String, String> _, ApiClient Function(String) _) =>
    (res.bodyBytes, [_sid('', 0)]);

// Cart write: Go er notun `cart_key` `__Host-cart` e (body theke muche), na hole guest er ager key abar (sliding 30
// din, DB er `updated_at` er sathe, 10.12 item 1).
(List<int>, List<String>)? _cart(http.Response res, Map<String, String> cookies, ApiClient Function(String) _) {
  if (res.statusCode < 200 || res.statusCode >= 300) return (res.bodyBytes, const []);
  try {
    if (jsonDecode(res.body) case final Map<String, dynamic> m when m.containsKey('cart_key')) {
      final k = m.remove('cart_key');
      // Login e key lage na (account cart), tai cookie na (10.12 item 8).
      if (validSid(cookies) != null) return (utf8.encode(jsonEncode(m)), const <String>[]);
      if (k is String && _cartKey.hasMatch(k)) return (utf8.encode(jsonEncode(m)), [_cartCookie(k, _month)]);
      return null;
    }
  } on FormatException {
    // JSON na: hubohu
  }
  final old = validSid(cookies) != null ? null : validCartKey(cookies);
  return (res.bodyBytes, [if (old != null) _cartCookie(old, _month)]);
}

/// Request er cookie (naam → value).
Map<String, String> parseCookies(String? header) => {
  for (final c in (header ?? '').split(';'))
    if (c.indexOf('=') case final i when i > 0) c.substring(0, i).trim(): c.substring(i + 1).trim(),
};

/// BFF (10.9): `/_/...` Go API te forward; baki [inner] e. Dam ba stock hishab na, shudhu forward (Rule 1).
Middleware bff(Config cfg, http.Client client, List<BffRoute> routes) =>
    (inner) => (request) async {
      final path = '/${request.url.path}';
      if (!path.startsWith('/_/')) return inner(request);
      final host = request.headers['host'] ?? '';
      if (request.method != 'GET' && request.method != 'HEAD' && !_sameOrigin(request.headers, host)) {
        return _error(403, 'forbidden');
      }
      final (route, match) = _find(routes, request.method, path);
      if (route == null) return _error(404, 'not_found');

      final cookies = parseCookies(request.headers['cookie']);
      final referer = _sameOriginPath(request.headers['referer'], host);
      final form = (request.headers['content-type'] ?? '').startsWith('application/x-www-form-urlencoded');
      final bytes = <int>[];
      await for (final chunk in request.read()) {
        bytes.addAll(chunk);
        if (bytes.length > _maxBody) return _error(413, 'validation_failed');
      }
      final String raw;
      final Map<String, Object>? fields;
      try {
        raw = utf8.decode(bytes);
        fields = form ? <String, Object>{...Uri.splitQueryString(raw)} : null;
        if (fields != null) {
          for (final k in route.ints) {
            if (int.tryParse('${fields[k] ?? ''}') case final n?) fields[k] = n;
          }
        }
      } on FormatException {
        return _error(400, 'validation_failed');
      } on ArgumentError {
        return _error(400, 'validation_failed');
      }
      final sid = validSid(cookies);
      // Login e Go key dekhe na (7.4); verify e lage (merge), purono sid thakleo.
      final cartKey = sid == null || route.session == _login ? validCartKey(cookies) : null;
      final api = http.Request(route.apiMethod ?? request.method, cfg.apiBaseUrl.resolve(route.api(match!)))
        ..headers.addAll({
          ...apiHeaders(request.headers, cfg),
          'Accept': 'application/json',
          'Accept-Language': localeOf(Uri.parse(referer).path).locale,
          if (sid != null) 'Authorization': 'Bearer $sid',
          'X-Cart-Key': ?cartKey,
          'Idempotency-Key': ?request.headers['idempotency-key'],
          if (form || raw.isNotEmpty) 'Content-Type': 'application/json; charset=utf-8',
        })
        ..body = fields != null ? jsonEncode(fields) : raw;

      final http.Response res;
      try {
        res = await client.send(api).then(http.Response.fromStream).timeout(const Duration(seconds: 15));
      } on TimeoutException {
        return _error(504, 'timeout');
      } on http.ClientException {
        return _error(502, 'network_error');
      }
      ApiClient withToken(String token) =>
          ApiClient(cfg.apiBaseUrl, client, headers: apiHeaders(request.headers, cfg), token: token);
      final out = route.session == null
          ? (res.bodyBytes, const <String>[])
          : await route.session!(res, cookies, withToken);
      if (out == null) return _error(502, 'internal');
      final (body, routeCookies) = out;
      // Session expire/revoke: cookie muche, na hole protiti page e `401` (10.12 item 7).
      final expired = res.statusCode == 401 && sid != null;
      final setCookies = [
        ...routeCookies,
        if (expired && !routeCookies.any((c) => c.startsWith('__Host-sid='))) _sid('', 0),
      ];
      // Session/cart cookie thaka uttor personal (10.3).
      final personal = <String, Object>{
        if (setCookies.isNotEmpty) 'set-cookie': setCookies,
        'cache-control': setCookies.isEmpty ? 'no-store' : 'private, no-store',
      };
      if (form) {
        // POST-redirect-GET; error hole form er page e `?error=<code>` (10.9 niyom 4)
        final back = Uri.parse(referer);
        final ok = res.statusCode >= 200 && res.statusCode < 300;
        final done = route.formDone;
        final locale = localeOf(back.path).locale;
        final to = ok
            ? (done == null ? back : Uri.parse(localePath(locale, done)))
            : res.statusCode == 401
            ? Uri.parse(localePath(locale, '/login')).replace(queryParameters: {'next': '$back'})
            : back.replace(queryParameters: {...back.queryParameters, 'error': _code(res.body)});
        return Response(303, headers: {'location': to.toString(), ...personal});
      }
      return Response(
        res.statusCode,
        body: body,
        headers: {
          'content-type': ?res.headers['content-type'],
          'retry-after': ?res.headers['retry-after'],
          ...personal,
        },
      );
    };

// CSRF (10.9 niyom 1): Origin thakle seta hubohu ei site er origin (scheme shoho, [originOf]); na thakle ba
// `null` (no-referrer page er form, 10.1) hole Sec-Fetch-Site same-origin.
bool _sameOrigin(Map<String, String> headers, String host) {
  final origin = headers['origin'];
  if (origin != null && origin != 'null') return host.isNotEmpty && origin == originOf(headers);
  return headers['sec-fetch-site'] == 'same-origin';
}

// Nijer page er path+query (redirect ar bhasha); onno host ba nai hole `/` (open redirect na).
String _sameOriginPath(String? referer, String host) {
  final u = Uri.tryParse(referer ?? '');
  // `//evil.com` ba `\` path browser e onno host (protocol-relative).
  if (u == null || u.authority != host || host.isEmpty || u.path.startsWith('//') || u.path.contains(r'\')) {
    return '/';
  }
  return u.hasQuery ? '${u.path}?${u.query}' : (u.path.isEmpty ? '/' : u.path);
}

(BffRoute?, Match?) _find(List<BffRoute> routes, String method, String path) {
  for (final r in routes) {
    if (!r.methods.contains(method)) continue;
    if (r.path.firstMatch(path) case final m?) return (r, m);
  }
  return (null, null);
}

String _code(String body) {
  try {
    final code = (jsonDecode(body) as Map)['error']['code'];
    if (code is String) return code;
  } catch (_) {}
  return 'internal';
}

Response _error(int status, String code) => Response(
  status,
  body: jsonEncode({
    'error': {'code': code, 'message': ''},
  }),
  headers: {'content-type': 'application/json', 'cache-control': 'no-store'},
);
