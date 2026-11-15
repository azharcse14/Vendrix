import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/product_card.dart';
import '../components/site.dart';
import '../components/wishlist_button.dart';
import '../seo/meta.dart';
import 'cart.dart';
import 'status.dart';

/// `/account/wishlist` (10.1, 10.12 item 3): login chay (guest er wishlist `wl` cookie e, page nai); `GET
/// /api/wishlist` er card, protitir niche bad dewar button ([WishlistButton]). Login na ba expire e `303 /login`.
class WishlistPage extends AsyncStatelessComponent {
  /// WishlistPage.
  const WishlistPage({super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    Component login() {
      final next = Uri(queryParameters: {'next': site.href('/account/wishlist')}).query;
      context.setHeader('location', '${site.href('/login')}?$next');
      context.setStatusCode(303);
      return const Component.empty();
    }

    final api = personalApi(context);
    if (api == null || api.token == null) return login();
    final List<ProductCard> items;
    try {
      items = await WishlistRepository(api).wishlist();
    } on ApiError catch (e) {
      if (e.status == 401) return login();
      return pageError(context, e);
    }
    return div(classes: 'px-4 py-6', [
      SeoHead(title: site.t('wishlist.title'), noindex: true),
      h1(classes: 'mb-4 font-heading text-2xl', [.text(site.t('wishlist.title'))]),
      if (items.isEmpty)
        p(classes: 'text-muted', [.text(site.t('wishlist.empty'))])
      else
        ul(classes: 'grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-4', [
          for (final c in items)
            li(classes: 'space-y-2', [
              ProductCardView(c),
              wishlistToggle(site, c.id, saved: true),
            ]),
        ]),
      const WishlistButton(loggedIn: true),
    ]);
  }
}
