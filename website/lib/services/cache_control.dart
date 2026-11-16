import 'package:shelf/shelf.dart';

import 'bff.dart';

// Ei cookie thakle page personal (session, cart, draft theme), Cloudflare e cache hole onno jon dekhbe (10.3).
// `wl` na: server `wl` diye kichu render kore na, product page public thake (10.12 item 3).
const _personal = {'__Host-sid', '__Host-cart', 'preview'};

// Token page (10.1): URL e token, page e order er data.
final _tokenPage = RegExp(r'^(en/)?(order|confirm|invoice|print)/');

// `/track` er uttor e order er status (10.13 item 6).
final _track = RegExp(r'^(en/)?track$');

/// Token page (`/order/`, `/confirm/`, `/invoice/`, `/print/`, `/en` shoho); [path] shelf er `url.path` (`/` chhara).
bool isTokenPage(String path) => _tokenPage.hasMatch(path);

/// `/track` ba `/en/track`; [path] shelf er `url.path`.
bool isTrackPage(String path) => _track.hasMatch(path);

/// `Cache-Control` (10.3, 14.2 test 24): personal cookie ba `Set-Cookie` → `private, no-store` (page er header
/// bodle, BFF o) ar token page (`/order/`, `/confirm/`, `/invoice/`, `/print/`) ar `/track` → `private, no-store`; BFF `/_/...` → `no-store`; baki page nije dile thake, na dile 2xx e `public, max-age=60`, onno `no-store`.
Middleware cacheControl() =>
    (inner) => (request) async {
      final res = await inner(request);
      final String value;
      if (parseCookies(request.headers['cookie']).keys.any(_personal.contains) ||
          res.headers.containsKey('set-cookie') ||
          isTokenPage(request.url.path) ||
          isTrackPage(request.url.path)) {
        value = 'private, no-store';
      } else if (request.url.path.startsWith('_/')) {
        value = 'no-store';
      } else if (res.headers['cache-control'] case final own?) {
        value = own;
      } else {
        value = res.statusCode >= 200 && res.statusCode < 300 ? 'public, max-age=60' : 'no-store';
      }
      return res.change(headers: {'cache-control': value});
    };
