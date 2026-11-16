import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/icon.dart';
import '../components/site.dart';
import '../seo/meta.dart';
import '../services/bff.dart';
import 'cart.dart';
import 'status.dart';

// Go er `rand.Text()`/confirm token er okkhor; onno kichu hole API call na (Go er onno path e jay na).
final _tokenChars = RegExp(r'^[A-Za-z0-9_-]+$');

/// Token e bhul okkhor hole 404 page (API call na); na hole null.
Component? badToken(BuildContext context, String token) {
  if (_tokenChars.hasMatch(token)) return null;
  context.setStatusCode(404);
  return const StatusPage(StatusKind.notFound);
}

/// Token page er JS chhara form er `action`: `en` e `?lang=en` (no-referrer e BFF Referer pay na, 10.13 item 7).
String tokenFormAction(Site site, String path) => site.locale == 'en' ? '$path?lang=en' : path;

/// Order er line (snapshot er naam/variant, qty, line total; Rule 1: taka API er).
class OrderItems extends StatelessComponent {
  /// Line.
  final List<OrderItem> items;

  /// OrderItems.
  const OrderItems(this.items, {super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    return ul(classes: 'divide-y divide-line', [
      for (final x in items)
        li(classes: 'flex justify-between gap-2 py-2', [
          span([
            .text(x.name.of(site.locale)),
            if (x.variantLabel.of(site.locale).isNotEmpty)
              span(classes: 'text-sm text-muted', [.text(' (${x.variantLabel.of(site.locale)})')]),
            .text(' × ${site.number(x.quantity)}'),
          ]),
          span([.text(site.taka(x.lineTotal))]),
        ]),
    ]);
  }
}

/// Ek sari: lekha ar man.
Component row(String label, String value) => rowOf(label, .text(value));

/// Ek sari, man component (badge).
Component rowOf(String label, Component value, {String classes = 'py-1'}) =>
    div(classes: 'flex items-center justify-between gap-4 $classes', [
      dt(classes: 'text-muted', [.text(label)]),
      dd(classes: 'text-right', [value]),
    ]);

/// Order status badge (10.5 v1.38): lekha shob shomoy, rong [orderStatusTone] theke. Puro class literal (Tailwind purge).
Component statusBadge(Site site, String status) => span(
  classes: switch (orderStatusTone(status)) {
    BadgeTone.success => 'badge badge-success',
    BadgeTone.danger => 'badge badge-danger',
    BadgeTone.warning => 'badge badge-warning',
    BadgeTone.accent => 'badge badge-accent',
    BadgeTone.muted => 'badge badge-muted',
  },
  [.text(site.t('order.status.$status'))],
);

/// `/order/{token}` ar `/order/{token}/done` (10.1, 10.6, 10.13 item 4): `GET /api/orders/{token}` er mask kora data
/// (8.19 item 6). Order page e `can_cancel` hole JS chhara cancel form (`/_/orders/{token}/cancel`); done page e
/// number, SMS er kotha (confirm button na, 8.10). Header (`no-referrer`, `noindex`, `private, no-store`) middleware e.
class OrderPage extends AsyncStatelessComponent {
  /// `public_token` (Rule 11).
  final String token;

  /// URL query (BFF er `error`).
  final Map<String, String> query;

  /// Done page.
  final bool done;

  /// OrderPage.
  const OrderPage(this.token, this.query, {this.done = false, super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    if (badToken(context, token) case final bad?) return bad;
    final Order o;
    try {
      o = await OrderRepository(site.catalog.api).byToken(token);
    } catch (e) {
      return pageError(context, e);
    }
    if (done) {
      return div(classes: 'mx-auto max-w-screen-sm space-y-6 px-4 py-8 text-center', [
        SeoHead(title: site.t('order.done.title'), noindex: true),
        span(classes: 'mx-auto flex size-16 items-center justify-center rounded-full bg-success-soft text-success', [
          const Icon(Icons.check, classes: 'size-8'),
        ]),
        h1([.text(site.t('order.done.title'))]),
        div(classes: 'card', [
          p(classes: 'mx-auto text-sm text-muted', [.text(site.t('track.number'))]),
          p(classes: 'mx-auto font-heading text-3xl tracking-wide', [.text(o.orderNumber)]),
        ]),
        // `pending` COD = confirm lagbe (8.7); link shudhu SMS e (8.10).
        p(
          classes: 'notice max-w-none text-left',
          attributes: {'role': 'status'},
          [
            .text(site.t(o.displayStatus == 'pending' ? 'order.done.confirm_sms' : 'order.done.sms')),
          ],
        ),
        div(classes: 'flex flex-col gap-3 sm:flex-row sm:justify-center', [
          a(href: site.href('/order/$token'), classes: 'btn btn-primary', [.text(site.t('order.done.view'))]),
          a(href: site.href('/'), classes: 'btn btn-secondary', [.text(site.t('page.home_link'))]),
        ]),
        if (validSid(requestCookies(context)) == null)
          p(classes: 'mx-auto text-sm text-muted', [.text(site.t('order.done.signup'))]),
      ]);
    }
    final error = query['error'];
    final deadline = o.deliveryDeadline;
    final cancelled = o.displayStatus == 'cancelled';
    return div(classes: 'mx-auto max-w-screen-sm space-y-6 px-4 py-8', [
      SeoHead(title: site.t('order.title', {'number': o.orderNumber}), noindex: true),
      div(classes: 'space-y-2', [
        h1([
          .text(site.t('order.title', {'number': o.orderNumber})),
        ]),
        statusBadge(site, o.displayStatus),
      ]),
      if (error != null)
        p(classes: 'notice notice-danger max-w-none', attributes: {'role': 'alert'}, [.text(site.error(error))]),
      if (cancelled)
        p(classes: 'notice max-w-none', attributes: {'role': 'status'}, [.text(site.t('order.cancelled'))]),
      div(classes: 'text-sm text-muted', [
        p([
          .text(site.t('order.placed_at', {'date': site.date(o.placedAt)})),
        ]),
        if (deadline != null && !cancelled)
          p([
            .text(site.t('order.deadline', {'date': site.date(deadline)})),
          ]),
      ]),
      dl(classes: 'card', [
        row(site.t('order.payment'), site.t('order.payment.${o.paymentMethod}')),
        row(site.t('order.customer'), '${o.customerName}, ${o.phoneMasked}'),
        row(site.t('order.recipient'), '${o.recipientName}, ${o.recipientPhoneMasked}'),
        row(site.t('order.address'), '${o.area}, ${o.district}'),
      ]),
      section(classes: 'card', [
        h2(classes: 'mb-2 text-xl', [.text(site.t('order.items'))]),
        OrderItems(o.items),
        dl(classes: 'mt-2 border-t border-line pt-2', [
          row(site.t('checkout.subtotal'), site.taka(o.subtotal)),
          if (o.discount > 0) row(site.t('checkout.discount'), site.taka(-o.discount)),
          row(site.t('checkout.delivery_charge'), site.taka(o.deliveryCharge)),
          rowOf(
            site.t('checkout.total'),
            span(classes: 'font-heading text-xl', [.text(site.taka(o.total))]),
            classes: 'mt-1 border-t border-line pt-2',
          ),
        ]),
      ]),
      if (o.canCancel)
        form(
          action: tokenFormAction(site, '/_/orders/$token/cancel'),
          method: FormMethod.post,
          classes: 'space-y-3',
          [
            // Reason oichchhik, tai bondho; submit baire (na khulei cancel).
            details([
              summary(classes: 'flex min-h-11 cursor-pointer items-center text-sm text-muted', [
                .text(site.t('order.cancel_reason')),
              ]),
              input(
                type: InputType.text,
                name: 'reason',
                classes: 'input',
                attributes: {'maxlength': '200', 'aria-label': site.t('order.cancel_reason')},
              ),
            ]),
            button(type: ButtonType.submit, classes: 'btn btn-danger w-full', [.text(site.t('order.cancel'))]),
          ],
        ),
    ]);
  }
}
