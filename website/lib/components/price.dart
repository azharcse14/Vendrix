import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import 'site.dart';

/// Dam (Rule 1: API ja dey tai, hishab na): dam heading font e (body font e 500 nai, 9.6), `compare_at_price` thakle
/// kete, kg er dam hole "/ kg".
class Price extends StatelessComponent {
  /// Poisha.
  final int price;

  /// Ager dam (poisha).
  final int? compareAt;

  /// `variable_weight`: dam 1 kg er (8.5).
  final bool perKg;

  /// Price.
  const Price(this.price, {this.compareAt, this.perKg = false, super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    return p(classes: 'flex flex-wrap items-baseline gap-x-2', [
      span(classes: 'font-heading text-lg font-medium text-ink', [
        .text(site.taka(price)),
        if (perKg) span(classes: 'font-body text-sm font-normal text-muted', [.text(' ${site.t('product.per_kg')}')]),
      ]),
      if (compareAt case final old?) s(classes: 'text-sm text-muted', [.text(site.taka(old))]),
    ]);
  }
}
