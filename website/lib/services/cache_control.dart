import 'package:shelf/shelf.dart';

import 'bff.dart';

// Ei cookie thakle page personal (session, cart, draft theme), Cloudflare e cache hole onno jon dekhbe (10.3).
const _personal = {'__Host-sid', '__Host-cart', 'preview'};

/// `Cache-Control` (10.3, 14.2 test 24): personal cookie ba `Set-Cookie` → `private, no-store` (page er header
/// bodle); BFF `/_/...` → `no-store`; baki page nije dile thake, na dile 2xx e `public, max-age=60`, onno `no-store`.
Middleware cacheControl() =>
    (inner) => (request) async {
      final res = await inner(request);
      final String value;
      if (request.url.path.startsWith('_/')) {
        value = 'no-store';
      } else if (parseCookies(request.headers['cookie']).keys.any(_personal.contains) ||
          res.headers.containsKey('set-cookie')) {
        value = 'private, no-store';
      } else if (res.headers['cache-control'] case final own?) {
        value = own;
      } else {
        value = res.statusCode >= 200 && res.statusCode < 300 ? 'public, max-age=60' : 'no-store';
      }
      return res.change(headers: {'cache-control': value});
    };
