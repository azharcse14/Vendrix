import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../pages/cart.dart';
import '../pages/policy.dart';
import '../services/locale.dart';
import 'icon.dart';
import 'site.dart';

/// Page er kaatamo (10.5): header, content (max 1280px), footer; mobile e bottom nav.
class Layout extends StatelessComponent {
  /// Page.
  final Component child;

  /// Layout.
  const Layout({required this.child, super.key});

  @override
  Component build(BuildContext context) {
    // Print page kagoj: header/footer/nav ar cart count call na (11.6).
    if (Uri.parse(context.url).path.startsWith('/print/')) return main_(classes: 'mx-auto max-w-screen-md', [child]);
    // Product page e bottom nav er bodole page er sticky bar (10.5 niyom 1).
    final product = localeOf(Uri.parse(context.url).path).path.startsWith('/p/');
    // Badge er halka count, page er data call er shathe (10.12 item 5); ignore: badge-i await kore.
    final count = switch (personalApi(context)) {
      final api? => CartRepository(api).count()..ignore(),
      null => null,
    };
    // Mobile e bottom nav ba sticky bar er jayga (h-14 + safe area).
    return CartLoad(
      count: count,
      child: div(classes: 'flex min-h-screen flex-col pb-[calc(3.5rem+env(safe-area-inset-bottom))] md:pb-0', [
        const _Header(),
        main_(classes: 'mx-auto w-full max-w-screen-xl flex-1', [child]),
        const _Footer(),
        if (!product) const _BottomNav(),
      ]),
    );
  }
}

// Cart er alada line er shonkhya (10.12 item 5); cookie chhara ba bhul (`401` o: protiti page redirect na) hole
// khali `hidden` (sob guest er page ekoi, cache thake), `AddToCart` JS e bhore. Icon er kone (relative parent).
class _CartBadge extends AsyncStatelessComponent {
  const _CartBadge();

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    int? n;
    try {
      n = await CartLoad.of(context);
    } catch (_) {
      // Badge er jonno page bhange na; `/cart` nije error dekhay.
    }
    final show = n != null && n > 0;
    return span(
      classes:
          'absolute -right-2 -top-1.5 min-w-4 rounded-full bg-primary px-1 text-center text-xs leading-4 '
          'text-on-primary',
      attributes: {'data-cart-badge': '', if (!show) 'hidden': ''},
      [if (show) .text(site.number(n))],
    );
  }
}

// Ekhon er page er link e `aria-current="page"` (10.7 v1.38); `path` bhasha chhara.
Map<String, String> _current(String path, String target) => {if (path == target) 'aria-current': 'page'};

// Logo na thakle heading font e naam + leaf (10.5 v1.38 item 4).
Component _brand(Site site, {String size = 'text-xl'}) {
  final name = site.settings.name.of(site.locale);
  final logoKey = site.settings.logoKey;
  final logo = logoKey == null ? null : site.images.url(logoKey, 320);
  if (logo != null) return img(src: logo, alt: name, height: 40, classes: 'h-10 w-auto');
  return span(classes: 'flex items-center gap-1.5 font-heading $size text-primary', [
    const Icon(Icons.leaf, classes: 'size-6 shrink-0'),
    .text(name),
  ]);
}

class _Header extends StatelessComponent {
  // Cart header e shob shomoy (product page e bottom nav nai, plan 4); Account mobile e bottom nav e.
  const _Header();

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final path = localeOf(Uri.parse(context.url).path).path;
    const link = 'flex min-h-11 items-center gap-1.5 rounded px-2 text-ink hover:text-primary';
    return header(classes: 'border-b border-line bg-surface', [
      div(classes: 'mx-auto flex max-w-screen-xl flex-wrap items-center gap-x-2 gap-y-2 px-4 py-2 md:gap-x-4', [
        a(href: site.href('/'), classes: 'flex min-h-11 items-center', [_brand(site)]),
        // JS chhara GET form (10.2); mobile e niche puro width.
        form(
          action: site.href('/search'),
          method: FormMethod.get,
          classes: 'order-last flex w-full gap-2 md:order-none md:w-auto md:flex-1',
          attributes: {'role': 'search'},
          [
            input(
              type: InputType.search,
              name: 'q',
              classes: 'input',
              attributes: {'placeholder': site.t('search.placeholder'), 'aria-label': site.t('search.placeholder')},
            ),
            button(
              type: ButtonType.submit,
              classes: 'btn btn-primary px-3',
              attributes: {'aria-label': site.t('search.submit')},
              [const Icon(Icons.search)],
            ),
          ],
        ),
        nav(
          classes: 'ml-auto flex items-center md:ml-0',
          attributes: {'aria-label': site.t('nav.main')},
          [
            a(href: '${site.href('/')}#categories', classes: 'hidden lg:flex $link', [
              .text(site.t('nav.category')),
            ]),
            if (site.hasEnglish)
              a(
                href: localePath(site.locale == 'en' ? 'bn' : 'en', path),
                classes: '$link text-sm',
                attributes: {'hreflang': site.locale == 'en' ? 'bn' : 'en'},
                [.text(site.t('lang.switch'))],
              ),
            a(href: site.href('/account'), classes: 'hidden md:flex $link', attributes: _current(path, '/account'), [
              const Icon(Icons.account),
              .text(site.t('nav.account')),
            ]),
            a(href: site.href('/cart'), classes: link, attributes: _current(path, '/cart'), [
              span(classes: 'relative', [const Icon(Icons.cart), const _CartBadge()]),
              // Badge icon er dan e 8px beriye thake, tai lekha ektu dure.
              span(classes: 'sr-only md:not-sr-only md:ml-2', [.text(site.t('nav.cart'))]),
            ]),
          ],
        ),
      ]),
    ]);
  }
}

// Cart M5 e (10.10 item 6, badge 10.12 item 5); Account M4 e (v1.19). Icon + lekha (Bangla min 14px, 9.6), active pill.
class _BottomNav extends StatelessComponent {
  const _BottomNav();

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final path = localeOf(Uri.parse(context.url).path).path;
    Component item(String target, Icons icon, String key, [Component? badge]) {
      final on = path == target;
      return a(
        href: target.startsWith('#') ? '${site.href('/')}$target' : site.href(target),
        classes:
            'flex min-h-11 min-w-0 flex-1 flex-col items-center justify-center gap-0.5 text-sm '
            '${on ? 'text-primary' : 'text-muted'}',
        attributes: _current(path, target),
        [
          span(classes: 'relative rounded-full px-4 py-0.5 ${on ? 'bg-primary-soft' : ''}', [Icon(icon), ?badge]),
          .text(site.t(key)),
        ],
      );
    }

    return nav(
      classes:
          'fixed inset-x-0 bottom-0 flex h-[calc(3.5rem+env(safe-area-inset-bottom))] border-t border-line '
          'bg-surface pb-[env(safe-area-inset-bottom)] md:hidden',
      attributes: {'aria-label': site.t('nav.bottom')},
      [
        item('/', Icons.home, 'nav.home'),
        item('#categories', Icons.category, 'nav.category'),
        item('/cart', Icons.cart, 'nav.cart', const _CartBadge()),
        item('/account', Icons.account, 'nav.account'),
      ],
    );
  }
}

// 3 column (md) ba stack: brand + DBID/BFSA, contact, policy (plan 4). Puro compliance (DBID page, complaint) M11 e.
class _Footer extends StatelessComponent {
  const _Footer();

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final s = site.settings;
    const title = 'mb-2 font-heading text-ink';
    return footer(classes: 'mt-12 border-t border-line bg-surface', [
      div(classes: 'mx-auto grid max-w-screen-xl gap-8 px-4 py-8 text-sm text-muted md:grid-cols-3', [
        div(classes: 'space-y-2', [
          _brand(site, size: 'text-lg'),
          if (s.dbidNumber case final n?)
            p([
              .text(site.t('footer.dbid', {'number': n})),
            ]),
          if (s.bfsaNumber case final n?)
            p([
              .text(site.t('footer.bfsa', {'number': n})),
            ]),
        ]),
        if (s.phone != null || s.email != null)
          div([
            p(classes: title, [.text(site.t('footer.contact'))]),
            if (s.phone case final phone?)
              a(href: 'tel:$phone', classes: 'flex min-h-11 items-center gap-2 underline', [
                const Icon(Icons.phone, classes: 'size-4 shrink-0'),
                .text(phone),
              ]),
            if (s.email case final email?)
              a(href: 'mailto:$email', classes: 'flex min-h-11 items-center underline', [.text(email)]),
          ]),
        nav(
          attributes: {'aria-label': site.t('policy.nav')},
          [
            p(classes: title, [.text(site.t('policy.nav'))]),
            for (final kind in policyKinds)
              a(href: site.href('/policy/$kind'), classes: 'flex min-h-11 items-center underline', [
                .text(site.t('policy.$kind')),
              ]),
          ],
        ),
      ]),
    ]);
  }
}
