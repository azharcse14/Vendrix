import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/icon.dart';
import '../components/site.dart';
import '../seo/meta.dart';
import 'cart.dart';
import 'order.dart';
import 'status.dart';

/// `/account/orders` (10.13 item 5): login chay; `GET /api/me/orders` (paged, `?page=`) er card, protitir
/// `/order/{public_token}` link. Login na ba expire e `303 /login`.
class AccountOrdersPage extends AsyncStatelessComponent {
  /// URL query (`page`).
  final Map<String, String> query;

  /// AccountOrdersPage.
  const AccountOrdersPage(this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    Component login() => toLogin(context, site.href('/account/orders'));

    final api = personalApi(context);
    if (api == null || api.token == null) return login();
    final n = int.tryParse(query['page'] ?? '');
    final Page<OrderSummary> list;
    try {
      list = await OrderRepository(api).myOrders(page: n != null && n > 1 ? n : null);
    } on ApiError catch (e) {
      if (e.status == 401) return login();
      return pageError(context, e);
    }
    String pageHref(int p) => p == 1 ? site.href('/account/orders') : '${site.href('/account/orders')}?page=$p';
    return div(classes: 'mx-auto max-w-screen-sm px-4 py-6', [
      SeoHead(title: site.t('account.orders'), noindex: true),
      h1(classes: 'mb-4', [.text(site.t('account.orders'))]),
      if (list.items.isEmpty)
        // Empty state (listing er moto): icon, lekha, home.
        div(classes: 'flex flex-col items-center gap-4 py-12 text-center', [
          const Icon(Icons.truck, classes: 'size-10 text-muted'),
          p(classes: 'text-muted', [.text(site.t('account.orders.empty'))]),
          a(href: site.href('/'), classes: 'btn btn-secondary', [.text(site.t('page.home_link'))]),
        ])
      else
        ul(classes: 'space-y-3', [
          for (final o in list.items)
            li([
              // Puro card link; 360 e 2 sari: number + badge, tarikh + total.
              a(
                href: site.href('/order/${o.publicToken}'),
                classes: 'card grid grid-cols-[1fr_auto] items-center gap-x-4 gap-y-1 hover:border-primary',
                [
                  span(classes: 'font-heading', [.text(o.orderNumber)]),
                  statusBadge(site, o.displayStatus),
                  span(classes: 'text-sm text-muted', [.text(site.date(o.placedAt))]),
                  span(classes: 'text-right font-heading', [.text(site.taka(o.total))]),
                ],
              ),
            ]),
        ]),
      nav(
        classes: 'mt-6 flex justify-between gap-3',
        attributes: {'aria-label': site.t('page.nav')},
        [
          if (list.page > 1)
            a(href: pageHref(list.page - 1), classes: 'btn btn-secondary', [.text(site.t('page.prev'))]),
          if (list.hasNext)
            a(href: pageHref(list.page + 1), classes: 'btn btn-secondary', [.text(site.t('page.next'))]),
        ],
      ),
    ]);
  }
}
