import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';
import 'package:universal_web/web.dart' as web;

import 'site.dart';

/// Category/search er filter form (GET, JS chhara-o kaj kore, 10.2): sort, shudhu stock, dam (taka).
class FilterForm extends StatelessComponent {
  /// Form er `action` (ei page).
  final String action;

  /// Ekhon er value.
  final String? sort;

  /// Shudhu stock.
  final bool inStock;

  /// Dam (taka).
  final int? minTaka, maxTaka;

  /// FilterForm.
  const FilterForm({required this.action, this.sort, this.inStock = false, this.minTaka, this.maxTaka, super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    String l(String key) => site.t('category.$key');
    const field = 'min-h-11 w-full rounded border border-line bg-page px-3 text-base';
    // Page e duita form (sidebar, sheet), tai id na; input label er bhetore.
    return form(action: action, method: FormMethod.get, classes: 'space-y-4', [
      label(classes: 'block space-y-1', [
        span(classes: 'text-sm', [.text(l('sort'))]),
        select(name: 'sort', classes: field, [
          for (final s in const ['newest', 'price_asc', 'price_desc', 'popular'])
            option(value: s, selected: s == (sort ?? 'newest'), [.text(l('sort.$s'))]),
        ]),
      ]),
      label(classes: 'flex min-h-11 items-center gap-2', [
        input(type: InputType.checkbox, name: 'in_stock', value: '1', checked: inStock, classes: 'h-5 w-5'),
        .text(l('in_stock')),
      ]),
      div(classes: 'grid grid-cols-2 gap-2', [
        for (final (name, key, value) in [('min', 'min_price', minTaka), ('max', 'max_price', maxTaka)])
          label(classes: 'block space-y-1', [
            span(classes: 'text-sm', [.text(l(key))]),
            input(
              type: InputType.number,
              name: name,
              value: value?.toString(),
              classes: field,
              attributes: {'min': '0', 'inputmode': 'numeric'},
            ),
          ]),
      ]),
      div(classes: 'flex gap-2', [
        button(type: ButtonType.submit, classes: 'min-h-11 flex-1 rounded bg-primary px-4 text-on-primary', [
          .text(l('apply')),
        ]),
        a(href: action, classes: 'flex min-h-11 items-center px-4 underline', [.text(l('clear'))]),
      ]),
    ]);
  }
}

/// Mobile er filter (10.2, 10.5): "Filter" button browser er `<dialog>` (`showModal`: focus atkay, Esc e bondho)
/// khole, bhetore [FilterForm]. Lekha server e; JS shudhu [DialogToggle].
class FilterSheet extends StatelessComponent {
  /// [FilterForm] er.
  final FilterForm form;

  /// FilterSheet.
  const FilterSheet(this.form, {super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    return div([
      button(
        type: ButtonType.button,
        classes: 'min-h-11 rounded border border-line bg-surface px-4',
        attributes: {'data-dialog-open': 'filters'},
        [.text(site.t('category.filter'))],
      ),
      dialog(
        id: 'filters',
        classes:
            'mb-0 mt-auto w-full max-w-none rounded-t bg-surface p-4 '
            'pb-[calc(1rem+env(safe-area-inset-bottom))] backdrop:bg-black/40',
        attributes: {'aria-label': site.t('category.filter')},
        [
          div(classes: 'mb-2 flex justify-end', [
            button(
              type: ButtonType.button,
              classes: 'min-h-11 px-4 underline',
              attributes: {'data-dialog-close': 'filters'},
              [.text(site.t('category.close'))],
            ),
          ]),
          form,
        ],
      ),
      const DialogToggle('filters'),
    ]);
  }
}

/// Browser e `[data-dialog-open=<id>]` click → `<dialog id>` `showModal`, `[data-dialog-close=<id>]` → `close`.
/// Client er data shudhu id (10.2: admin er lekha HTML comment e jay na).
@client
class DialogToggle extends StatefulComponent {
  /// `<dialog>` er id.
  final String dialogId;

  /// DialogToggle.
  const DialogToggle(this.dialogId, {super.key});

  @override
  State<DialogToggle> createState() => _DialogToggleState();
}

class _DialogToggleState extends State<DialogToggle> {
  @override
  void initState() {
    super.initState();
    if (!kIsWeb) return;
    final id = component.dialogId;
    final d = web.document.getElementById(id) as web.HTMLDialogElement?;
    if (d == null) return;
    for (final (attr, action) in [('data-dialog-open', () => d.showModal()), ('data-dialog-close', () => d.close())]) {
      final nodes = web.document.querySelectorAll('[$attr="$id"]');
      for (var i = 0; i < nodes.length; i++) {
        web.EventStreamProviders.clickEvent.forTarget(nodes.item(i)).listen((_) => action());
      }
    }
  }

  @override
  Component build(BuildContext context) => const Component.empty();
}
