import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:shared/shared.dart';
import 'package:universal_web/web.dart' as web;

import 'price.dart';
import 'site.dart';

/// Variant er naam (option er value, jemon weight).
String variantLabel(Variant v) => v.options.values.join(' · ');

/// Prothom stock e thaka variant, na hole prothom ta.
Variant? defaultVariant(List<Variant> vs) =>
    vs.isEmpty ? null : vs.firstWhere((v) => v.inStock, orElse: () => vs.first);

/// Protiti variant er dam (Rule 1: API er dam), shudhu [selected] ta dekha jay; [VariantSwitch] bodlay. Price block e
/// discount ar stock shesh badge (lekha, rong only na; 10.7). [withLabel] hole (sticky bar) naam, badge er bodole
/// choto lekha.
class VariantPrices extends StatelessComponent {
  /// Variant.
  final List<Variant> variants;

  /// Shuru te kon ta.
  final Variant selected;

  /// `variable_weight`.
  final bool perKg;

  /// Naam o dekhao.
  final bool withLabel;

  /// VariantPrices.
  const VariantPrices(this.variants, this.selected, {this.perKg = false, this.withLabel = false, super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    return div([
      for (final v in variants)
        div(
          attributes: {'data-variant': '${v.id}', if (v.id != selected.id) 'hidden': ''},
          [
            if (withLabel && variants.length > 1) p(classes: 'text-sm text-muted', [.text(variantLabel(v))]),
            Price(v.price, compareAt: v.compareAtPrice, perKg: perKg),
            if (!withLabel)
              _badges(site, v)
            else if (!v.inStock)
              p(classes: 'text-sm text-muted', [.text(site.t('product.out_of_stock'))]),
          ],
        ),
    ]);
  }
}

Component _badges(Site site, Variant v) {
  final off = discountPercent(v.price, v.compareAtPrice);
  if (off == null && v.inStock) return const Component.empty();
  return div(classes: 'mt-1 flex flex-wrap gap-1', [
    if (off != null)
      span(classes: 'badge badge-accent', [
        .text(site.t('product.discount', {'percent': site.number(off)})),
      ]),
    if (!v.inStock) span(classes: 'badge badge-danger', [.text(site.t('product.out_of_stock'))]),
  ]);
}

/// Variant bachai (10.2): radio (JS chhara-o bachai hoy), lekha server e; dam bodlay [VariantSwitch].
class VariantPicker extends StatelessComponent {
  /// Variant (2 ba beshi).
  final List<Variant> variants;

  /// Shuru te kon ta.
  final Variant selected;

  /// VariantPicker.
  const VariantPicker(this.variants, this.selected, {super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    return fieldset(classes: 'space-y-2', [
      legend(classes: 'mb-2 text-sm', [.text(site.t('product.choose'))]),
      div(classes: 'flex flex-wrap gap-2', [
        for (final v in variants)
          label(
            // `.chip:has(:checked)` bachai kora ta primary-soft (JS chhara-o).
            classes: 'chip cursor-pointer has-[:focus-visible]:outline',
            [
              input(
                type: InputType.radio,
                // Cart e add er JS chhara form er field (10.12 item 4).
                name: 'variant_id',
                value: '${v.id}',
                checked: v.id == selected.id,
                classes: 'sr-only',
              ),
              span([.text(variantLabel(v))]),
              if (!v.inStock) span(classes: 'text-muted', [.text('(${site.t('product.out_of_stock')})')]),
            ],
          ),
      ]),
      const VariantSwitch(),
    ]);
  }
}

/// Browser e `input[name=variant_id]` bodlale shudhu oi `[data-variant=<id>]` dekhay. Client er data kichu na (10.2).
@client
class VariantSwitch extends StatefulComponent {
  /// VariantSwitch.
  const VariantSwitch({super.key});

  @override
  State<VariantSwitch> createState() => _VariantSwitchState();
}

class _VariantSwitchState extends State<VariantSwitch> {
  @override
  void initState() {
    super.initState();
    if (!kIsWeb) return;
    final radios = web.document.querySelectorAll('input[name="variant_id"]');
    for (var i = 0; i < radios.length; i++) {
      final radio = radios.item(i) as web.HTMLInputElement;
      web.EventStreamProviders.changeEvent.forTarget(radio).listen((_) {
        final nodes = web.document.querySelectorAll('[data-variant]');
        for (var j = 0; j < nodes.length; j++) {
          final n = nodes.item(j) as web.Element;
          n.toggleAttribute('hidden', n.getAttribute('data-variant') != radio.value);
        }
      });
    }
  }

  @override
  Component build(BuildContext context) => const Component.empty();
}
