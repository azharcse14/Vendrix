import 'package:http/http.dart' as http;
import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:shared/api/api_client.dart';
import 'package:universal_web/web.dart' as web;

import 'site.dart';

const _maxWl = 50;

/// `Cookie` header theke guest er wishlist (`wl`, 10.12 item 3): koma diye decimal ID, bhul ba 0 bad.
List<int> wlIds(String cookieHeader) {
  for (final c in cookieHeader.split(';')) {
    final t = c.trim();
    if (!t.startsWith('wl=')) continue;
    return <int>{
      for (final s in t.substring(3).split(','))
        if (int.tryParse(s) case final id? when id > 0) id,
    }.toList();
  }
  return [];
}

/// `wl` cookie (10.3): koma literal (BFF koma diye bhag kore), shesh 50 ta, 1 bochor; khali hole muche. HttpOnly
/// na, JS lekhe.
String wlCookie(List<int> ids) {
  final keep = ids.length > _maxWl ? ids.sublist(ids.length - _maxWl) : ids;
  return 'wl=${keep.join(',')}; Path=/; Max-Age=${keep.isEmpty ? 0 : 31536000}; Secure; SameSite=Lax';
}

/// Heart button (server render): [saved] shuru obostha (login e SSR, guest e JS cookie theke). JS chhara kaj kore
/// na, tai `hidden`; [WishlistButton] dekhay. Lekha attribute e (client e translator nai).
Component wishlistToggle(Site site, int productId, {required bool saved}) => button(
  type: ButtonType.button,
  classes: 'min-h-11 w-full rounded border border-line px-3 text-sm aria-pressed:border-primary',
  attributes: {
    'data-wishlist': '$productId',
    'aria-pressed': '$saved',
    'data-on': site.t('wishlist.remove'),
    'data-off': site.t('wishlist.add'),
    'hidden': '',
  },
  [.text(site.t(saved ? 'wishlist.remove' : 'wishlist.add'))],
);

/// Page er shob `button[data-wishlist]` chalu kore (10.12 item 3): guest e `wl` cookie (server e jay na, page
/// public thake); login e BFF `/_/wishlist/{id}` (POST/DELETE).
@client
class WishlistButton extends StatefulComponent {
  /// `__Host-sid` ache (server jane; HttpOnly, JS dekhe na).
  final bool loggedIn;

  /// WishlistButton.
  const WishlistButton({required this.loggedIn, super.key});

  @override
  State<WishlistButton> createState() => _WishlistButtonState();
}

class _WishlistButtonState extends State<WishlistButton> {
  late final ApiClient _bff = ApiClient(Uri.base, http.Client());

  @override
  void initState() {
    super.initState();
    if (!kIsWeb) return;
    final saved = component.loggedIn ? null : wlIds(web.document.cookie);
    final buttons = web.document.querySelectorAll('button[data-wishlist]');
    for (var i = 0; i < buttons.length; i++) {
      final b = buttons.item(i) as web.HTMLButtonElement;
      final id = int.tryParse(b.getAttribute('data-wishlist') ?? '');
      if (id == null) continue;
      if (saved != null) _set(b, saved.contains(id));
      b.removeAttribute('hidden');
      web.EventStreamProviders.clickEvent.forTarget(b).listen((_) => _toggle(b, id));
    }
  }

  void _set(web.HTMLButtonElement b, bool on) {
    b.setAttribute('aria-pressed', '$on');
    b.textContent = b.getAttribute(on ? 'data-on' : 'data-off');
  }

  Future<void> _toggle(web.HTMLButtonElement b, int id) async {
    final on = b.getAttribute('aria-pressed') != 'true';
    if (!component.loggedIn) {
      final ids = wlIds(web.document.cookie)..remove(id);
      if (on) ids.add(id);
      web.document.cookie = wlCookie(ids);
      _set(b, on);
      return;
    }
    if (b.disabled) return;
    b.disabled = true;
    _set(b, on);
    try {
      await _bff.send(on ? 'POST' : 'DELETE', '/_/wishlist/$id');
    } on ApiError catch (e) {
      _set(b, !on);
      // Session expire: abar login, tarpor ei page e.
      if (e.status == 401) {
        final path = web.window.location.pathname;
        final en = path == '/en' || path.startsWith('/en/');
        web.window.location.assign('${en ? '/en' : ''}/login?${Uri(queryParameters: {'next': path}).query}');
      }
    } finally {
      b.disabled = false;
    }
  }

  @override
  Component build(BuildContext context) => const Component.empty();
}
