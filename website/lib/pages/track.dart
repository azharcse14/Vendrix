import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/site.dart';
import '../seo/meta.dart';
import 'order.dart';
import 'status.dart';

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
      h1([.text(site.t('track.title'))]),
      if (error != null) p(classes: 'notice notice-danger', attributes: {'role': 'alert'}, [.text(error)]),
      if (order != null)
        section(
          id: 'track-result',
          classes: 'card space-y-3',
          attributes: {'role': 'status'},
          [
            h2(classes: 'text-xl', [.text(order.orderNumber)]),
            statusBadge(site, order.displayStatus),
            p(classes: 'text-sm text-muted', [
              .text(site.t('order.placed_at', {'date': site.date(order.placedAt)})),
            ]),
            h3(classes: 'pt-2 text-base', [.text(site.t('track.history'))]),
            // Timeline CSS only: dot `before:`, line `after:` ek dot theke porer dot porjonto (shesh step e na, tai
            // line shesh dot er niche jay na); shesh step `aria-current` e primary (rong chhara-o lekha ache).
            ol(classes: 'ml-1.5', [
              for (final (i, h) in order.history.indexed)
                li(
                  classes:
                      'relative pb-3 pl-5 text-sm text-muted last:pb-0 before:absolute before:-left-1.5 '
                      'before:top-1.5 before:size-3 before:rounded-full before:bg-line after:absolute after:-left-px '
                      'after:top-[18px] after:h-[calc(100%-12px)] after:w-0.5 after:bg-line last:after:hidden '
                      'aria-[current=step]:text-ink aria-[current=step]:before:bg-primary',
                  attributes: {if (i == order.history.length - 1) 'aria-current': 'step'},
                  [
                    span(classes: 'block', [.text(site.t('order.status.${h.status}'))]),
                    span(classes: 'block text-muted', [.text(site.date(h.at))]),
                  ],
                ),
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
            classes: 'input',
            attributes: {'required': '', 'maxlength': '32', 'autocomplete': 'off'},
          ),
        ]),
        label(classes: 'block space-y-1', [
          span(classes: 'text-sm', [.text(site.t('track.phone'))]),
          // Phone abar bhora hoy na (page e phone na thake).
          input(
            type: InputType.tel,
            name: 'phone',
            classes: 'input',
            attributes: {
              'inputmode': 'numeric',
              'autocomplete': 'tel',
              'required': '',
              if (badPhone) ...{'aria-invalid': 'true', 'aria-describedby': 'track-phone-error', 'autofocus': ''},
            },
          ),
          if (badPhone) span(id: 'track-phone-error', classes: 'text-sm', [.text(site.error('invalid_phone'))]),
        ]),
        button(type: ButtonType.submit, classes: 'btn btn-primary w-full', [
          .text(site.t('track.submit')),
        ]),
      ]),
    ]);
  }
}
