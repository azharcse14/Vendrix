import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/site.dart';
import '../seo/meta.dart';
import 'cart.dart';
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
    const link = 'inline-flex min-h-11 items-center underline';
    return div(classes: 'mx-auto max-w-screen-sm px-4 py-6', [
      SeoHead(title: site.t('account.orders'), noindex: true),
      h1(classes: 'mb-4 font-heading text-2xl', [.text(site.t('account.orders'))]),
      if (list.items.isEmpty)
        p(classes: 'text-muted', [.text(site.t('account.orders.empty'))])
      else
        ul(classes: 'divide-y divide-line', [
          for (final o in list.items)
            li(classes: 'flex flex-wrap items-baseline justify-between gap-2 py-3', [
              a(href: site.href('/order/${o.publicToken}'), classes: 'min-h-11 font-medium underline', [
                .text(o.orderNumber),
              ]),
              span([.text(site.t('order.status.${o.displayStatus}'))]),
              span(classes: 'text-sm text-muted', [.text(site.date(o.placedAt))]),
              span(classes: 'font-medium', [.text(site.taka(o.total))]),
            ]),
        ]),
      nav(
        classes: 'mt-4 flex justify-between',
        attributes: {'aria-label': site.t('page.nav')},
        [
          if (list.page > 1) a(href: pageHref(list.page - 1), classes: link, [.text(site.t('page.prev'))]),
          if (list.hasNext) a(href: pageHref(list.page + 1), classes: link, [.text(site.t('page.next'))]),
        ],
      ),
    ]);
  }
}
