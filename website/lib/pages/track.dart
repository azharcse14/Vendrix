import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/site.dart';
import '../seo/meta.dart';
import 'status.dart';

const _field = 'min-h-11 w-full rounded border border-line bg-page px-3 text-base';

/// `/track` (10.1, 10.13 item 6): JS chhara form nijer page e POST; `renderPage` form pore [Site.trackForm] e dey,
/// ei page Go `POST /api/orders/track` dake (phone body te, `X-Client-IP` shoho) ar uttor (status, itihash; order er
/// link na) ekoi response e. `404` → `track.not_found`, `429` hubohu + `Retry-After`, Go down → 503 page.
class TrackPage extends AsyncStatelessComponent {
  /// TrackPage.
  const TrackPage({super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final f = site.trackForm;
    TrackedOrder? order;
    String? error;
    var badPhone = false;
    if (f != null) {
      try {
        order = await OrderRepository(site.catalog.api).track(f.number, f.phone);
      } on ApiError catch (e) {
        switch (e.status) {
          case 404:
            context.setStatusCode(404);
            error = site.t('track.not_found');
          case 429:
            context.setStatusCode(429);
            if (e.retryAfter case final r?) context.setHeader('retry-after', '${r.inSeconds}');
            error = site.error(e.code);
          case 400:
            context.setStatusCode(400);
            badPhone = e.fields.containsKey('phone');
            if (!badPhone) error = site.error(e.code);
          default:
            return pageError(context, e);
        }
      } catch (e) {
        return pageError(context, e);
      }
    }
    return div(classes: 'mx-auto max-w-sm space-y-6 px-4 py-8', [
      SeoHead(title: site.t('track.title'), noindex: true),
      h1(classes: 'font-heading text-2xl', [.text(site.t('track.title'))]),
      if (error != null)
        p(classes: 'rounded border border-line bg-tint p-3 text-sm', attributes: {'role': 'alert'}, [.text(error)]),
      if (order != null)
        section(
          id: 'track-result',
          classes: 'space-y-2 rounded border border-line p-4',
          attributes: {'role': 'status'},
          [
            p(classes: 'text-lg font-medium', [.text(order.orderNumber)]),
            p([.text(site.t('order.status.${order.displayStatus}'))]),
            p(classes: 'text-sm text-muted', [
              .text(site.t('order.placed_at', {'date': site.date(order.placedAt)})),
            ]),
            h2(classes: 'pt-2 font-heading', [.text(site.t('track.history'))]),
            ol(classes: 'space-y-1 text-sm', [
              for (final h in order.history) li([.text('${site.date(h.at)}: ${site.t('order.status.${h.status}')}')]),
            ]),
          ],
        ),
      form(id: 'track', method: FormMethod.post, action: site.href('/track'), classes: 'space-y-4', [
        label(classes: 'block space-y-1', [
          span(classes: 'text-sm', [.text(site.t('track.number'))]),
          input(
            type: InputType.text,
            name: 'number',
            value: f?.number,
            classes: _field,
            attributes: {'required': '', 'maxlength': '32', 'autocomplete': 'off'},
          ),
        ]),
        label(classes: 'block space-y-1', [
          span(classes: 'text-sm', [.text(site.t('track.phone'))]),
          // Phone abar bhora hoy na (page e phone na thake).
          input(
            type: InputType.tel,
            name: 'phone',
            classes: _field,
            attributes: {
              'inputmode': 'numeric',
              'autocomplete': 'tel',
              'required': '',
              if (badPhone) ...{'aria-invalid': 'true', 'aria-describedby': 'track-phone-error', 'autofocus': ''},
            },
          ),
          if (badPhone) span(id: 'track-phone-error', classes: 'text-sm', [.text(site.error('invalid_phone'))]),
        ]),
        button(type: ButtonType.submit, classes: 'min-h-11 w-full rounded bg-primary px-4 text-on-primary', [
          .text(site.t('track.submit')),
        ]),
      ]),
    ]);
  }
}
