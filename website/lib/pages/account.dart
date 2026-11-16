import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/site.dart';
import '../seo/meta.dart';
import '../services/bff.dart';
import 'cart.dart';
import 'status.dart';

/// `/account` (10.1, 10.11 item 5): `__Host-sid` diye SSR e `GET /api/me`; naam ar SMS er bhasha (JS chhara form
/// `/_/account` e), logout. Session na thakle ba expire hole `303 /login?next=`. Cookie thake, tai `cacheControl`
/// `private, no-store` dey (10.3).
class AccountPage extends AsyncStatelessComponent {
  /// URL query (BFF er `error`).
  final Map<String, String> query;

  /// AccountPage.
  const AccountPage(this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    Component login() => toLogin(context, site.href('/account'));

    final token = validSid(requestCookies(context));
    if (token == null) return login();
    final api = site.catalog.api;
    final User me;
    try {
      me = await AccountRepository(
        ApiClient(api.baseUrl, api.client, locale: api.locale, headers: api.headers, token: token),
      ).me();
    } on ApiError catch (e) {
      if (e.status == 401) return login();
      return pageError(context, e);
    }
    const field = 'min-h-11 w-full rounded border border-line bg-page px-3 text-base';
    final error = query['error'];
    return div(classes: 'mx-auto max-w-sm space-y-6 px-4 py-8', [
      SeoHead(title: site.t('account.title'), noindex: true),
      h1(classes: 'font-heading text-2xl', [.text(site.t('account.title'))]),
      if (error != null)
        p(
          classes: 'rounded border border-line bg-tint p-3 text-sm',
          attributes: {'role': 'alert'},
          [
            .text(site.error(error)),
          ],
        ),
      p([
        .text(site.t('account.phone', {'phone': me.phone})),
      ]),
      a(href: site.href('/account/wishlist'), classes: 'flex min-h-11 items-center underline', [
        .text(site.t('wishlist.title')),
      ]),
      a(href: site.href('/account/orders'), classes: 'flex min-h-11 items-center underline', [
        .text(site.t('account.orders')),
      ]),
      form(action: '/_/account', method: FormMethod.post, classes: 'space-y-4', [
        label(classes: 'block space-y-1', [
          span(classes: 'text-sm', [.text(site.t('account.name'))]),
          input(
            type: InputType.text,
            name: 'name',
            value: me.name ?? '',
            classes: field,
            attributes: {'autocomplete': 'name', 'maxlength': '100'},
          ),
        ]),
        label(classes: 'block space-y-1', [
          span(classes: 'text-sm', [.text(site.t('account.locale'))]),
          select(name: 'locale', classes: field, [
            for (final l in const ['bn', 'en'])
              option(value: l, selected: l == me.locale, [.text(site.t('account.locale.$l'))]),
          ]),
        ]),
        button(type: ButtonType.submit, classes: 'min-h-11 w-full rounded bg-primary px-4 text-on-primary', [
          .text(site.t('account.save')),
        ]),
      ]),
      form(action: '/_/auth/logout', method: FormMethod.post, [
        button(type: ButtonType.submit, classes: 'min-h-11 w-full rounded border border-line px-4', [
          .text(site.t('account.logout')),
        ]),
      ]),
    ]);
  }
}
