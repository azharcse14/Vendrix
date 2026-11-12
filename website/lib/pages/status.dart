import 'dart:io';

import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/site.dart';
import '../seo/meta.dart';

/// Kon status page.
enum StatusKind {
  /// 404.
  notFound('page.not_found'),

  /// Shop bondho (503, 3.2 Niyom 2).
  closed('page.shop_closed'),

  /// API down ba bhul (503).
  error('page.error');

  /// Lekha key er shuru (`.title`, `.body`).
  final String key;
  const StatusKind(this.key);
}

/// 404, bondho ar error page (10.10 item 10). Status code [resolvePage] ba router (`NotFound`) dey.
class StatusPage extends StatelessComponent {
  /// Kon page.
  final StatusKind kind;

  /// StatusPage.
  const StatusPage(this.kind, {super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    return main_(classes: 'mx-auto max-w-screen-sm px-4 py-16 text-center', [
      SeoHead(title: site.t('${kind.key}.title'), noindex: true),
      h1([.text(site.t('${kind.key}.title'))]),
      p(classes: 'mt-4 text-muted', [.text(site.t('${kind.key}.body'))]),
      // Bondho shop e home o bondho, tai link na.
      if (kind != StatusKind.closed)
        a(
          href: site.href('/'),
          classes: 'mt-8 inline-flex min-h-11 items-center rounded bg-primary px-6 text-on-primary',
          [.text(site.t('page.home_link'))],
        ),
    ]);
  }
}

/// Page er API bhul (10.10 item 10): `not_found` → 404 page; baki (API down) → 503 error page, log.
Component pageError(BuildContext context, Object error) {
  if (error is ApiError && error.code == 'not_found') {
    context.setStatusCode(404);
    return const StatusPage(StatusKind.notFound);
  }
  stderr.writeln('page: $error');
  context.setStatusCode(503);
  return const StatusPage(StatusKind.error);
}
