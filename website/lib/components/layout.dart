import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';

import '../services/locale.dart';
import 'site.dart';

/// Page er kaatamo (10.5): header, content (max 1280px), footer; mobile e bottom nav.
class Layout extends StatelessComponent {
  /// Page.
  final Component child;

  /// Layout.
  const Layout({required this.child, super.key});

  @override
  Component build(BuildContext context) {
    // Mobile e bottom nav er jayga (h-14 + safe area).
    return div(classes: 'flex min-h-screen flex-col pb-[calc(3.5rem+env(safe-area-inset-bottom))] md:pb-0', [
      const _Header(),
      main_(classes: 'mx-auto w-full max-w-screen-xl flex-1', [child]),
      const _Footer(),
      const _BottomNav(),
    ]);
  }
}

class _Header extends StatelessComponent {
  const _Header();

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final path = localeOf(Uri.parse(context.url).path).path;
    final name = site.settings.name.of(site.locale);
    final logoKey = site.settings.logoKey;
    final logo = logoKey == null ? null : site.images.url(logoKey, 320);
    return header(classes: 'border-b border-line bg-surface', [
      div(classes: 'mx-auto flex max-w-screen-xl flex-wrap items-center gap-3 px-4 py-2', [
        a(href: site.href('/'), classes: 'flex min-h-11 items-center font-heading text-xl text-primary', [
          // Logo thakle chobi (alt = naam), na hole naam.
          if (logo != null) img(src: logo, alt: name, height: 40, classes: 'h-10 w-auto') else .text(name),
        ]),
        nav(
          classes: 'ml-auto hidden lg:block',
          attributes: {'aria-label': site.t('nav.main')},
          [
            a(href: '${site.href('/')}#categories', classes: 'flex min-h-11 items-center px-2', [
              .text(site.t('nav.category')),
            ]),
          ],
        ),
        if (site.hasEnglish)
          a(
            href: site.locale == 'en' ? path : (path == '/' ? '/en' : '/en$path'),
            classes: 'ml-auto flex min-h-11 items-center px-2 text-sm lg:ml-0',
            attributes: {'hreflang': site.locale == 'en' ? 'bn' : 'en'},
            [.text(site.t('lang.switch'))],
          ),
        // JS chhara GET form (10.2).
        form(
          action: site.href('/search'),
          method: FormMethod.get,
          classes: 'order-last flex w-full gap-2 md:order-none md:w-auto md:flex-1',
          attributes: {'role': 'search'},
          [
            input(
              type: InputType.search,
              name: 'q',
              classes: 'min-h-11 w-full rounded border border-line bg-page px-3 text-base',
              attributes: {'placeholder': site.t('search.placeholder'), 'aria-label': site.t('search.placeholder')},
            ),
            button(type: ButtonType.submit, classes: 'min-h-11 rounded bg-primary px-4 text-on-primary', [
              .text(site.t('search.submit')),
            ]),
          ],
        ),
      ]),
    ]);
  }
}

// Cart ar Account M4/M5 er page porjonto lukano (10.10 item 6).
class _BottomNav extends StatelessComponent {
  const _BottomNav();

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    Component item(String href, String key) => a(
      href: href,
      classes: 'flex min-h-11 flex-1 items-center justify-center',
      [.text(site.t(key))],
    );
    return nav(
      classes:
          'fixed inset-x-0 bottom-0 flex h-[calc(3.5rem+env(safe-area-inset-bottom))] border-t border-line '
          'bg-surface pb-[env(safe-area-inset-bottom)] md:hidden',
      attributes: {'aria-label': site.t('nav.main')},
      [
        item(site.href('/'), 'nav.home'),
        item('${site.href('/')}#categories', 'nav.category'),
      ],
    );
  }
}

// Policy link step 10 e, puro compliance M11 e.
class _Footer extends StatelessComponent {
  const _Footer();

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    final s = site.settings;
    return footer(classes: 'mt-12 border-t border-line bg-surface', [
      div(classes: 'mx-auto max-w-screen-xl space-y-2 px-4 py-8 text-sm text-muted', [
        if (s.phone != null || s.email != null)
          p([
            .text('${site.t('footer.contact')}: '),
            if (s.phone case final phone?) a(href: 'tel:$phone', classes: 'underline', [.text(phone)]),
            if (s.phone != null && s.email != null) .text(' · '),
            if (s.email case final email?) a(href: 'mailto:$email', classes: 'underline', [.text(email)]),
          ]),
        if (s.dbidNumber case final n?)
          p([
            .text(site.t('footer.dbid', {'number': n})),
          ]),
        if (s.bfsaNumber case final n?)
          p([
            .text(site.t('footer.bfsa', {'number': n})),
          ]),
      ]),
    ]);
  }
}
