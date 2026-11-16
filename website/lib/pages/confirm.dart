import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/site.dart';
import '../seo/meta.dart';
import 'order.dart';
import 'status.dart';

/// `/confirm/{confirm_token}` (8.10, 10.1): `GET /api/confirm/{token}` er shar-songkhep (mask); `pending` hole JS chhara
/// confirm form (`/_/confirm/{token}`), na hole obostha. Token SMS e chhara kothao na (Rule 11).
class ConfirmPage extends AsyncStatelessComponent {
  /// `confirm_token`.
  final String token;

  /// URL query (BFF er `error`).
  final Map<String, String> query;

  /// ConfirmPage.
  const ConfirmPage(this.token, this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    if (badToken(context, token) case final bad?) return bad;
    final ConfirmSummary o;
    try {
      o = await OrderRepository(site.catalog.api).confirmView(token);
    } catch (e) {
      return pageError(context, e);
    }
    final pending = o.displayStatus == 'pending';
    final error = query['error'];
    return div(classes: 'mx-auto max-w-screen-sm space-y-6 px-4 py-8', [
      SeoHead(title: site.t('confirm.title'), noindex: true),
      h1(classes: 'font-heading text-2xl', [.text(site.t('confirm.title'))]),
      if (error != null)
        p(
          classes: 'rounded border border-line bg-tint p-3 text-sm',
          attributes: {'role': 'alert'},
          [.text(site.error(error))],
        ),
      if (pending)
        p([.text(site.t('confirm.body'))])
      else if (o.displayStatus != 'cancelled')
        p(
          classes: 'rounded border border-line bg-tint p-3',
          attributes: {'role': 'status'},
          [.text(site.t('confirm.done'))],
        ),
      dl([
        row(site.t('track.number'), o.orderNumber),
        row(site.t('order.status'), site.t('order.status.${o.displayStatus}')),
        row(site.t('order.customer'), '${o.customerName}, ${o.phoneMasked}'),
        row(site.t('order.address'), '${o.area}, ${o.district}'),
        row(site.t('order.payment'), site.t('order.payment.${o.paymentMethod}')),
      ]),
      OrderItems(o.items),
      dl([row(site.t('checkout.total'), site.taka(o.total))]),
      if (pending)
        form(action: tokenFormAction(site, '/_/confirm/$token'), method: FormMethod.post, [
          button(type: ButtonType.submit, classes: 'min-h-11 w-full rounded bg-primary px-4 text-on-primary', [
            .text(site.t('confirm.submit')),
          ]),
        ]),
    ]);
  }
}
