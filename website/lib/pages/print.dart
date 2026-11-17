import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';

import '../components/site.dart';
import '../seo/meta.dart';
import '../services/print.dart';
import 'order.dart';
import 'status.dart';

/// Print er kind (11.6); ochena kind e route na, router er 404.
const printKinds = ['packing_slip', 'pick_list', 'invoice'];

// Go `SignPrint`: base64url payload + "." + base64url HMAC.
final _printToken = RegExp(r'^[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$');

/// `/print/{kind}?token=` (10.1, 11.6, 11.7 item 8): staff er packing slip, pick list, invoice; A4, `?paper=80mm`
/// thermal. Client JS na (Ctrl+P), site er header/footer na (`Layout`); header middleware e, script `renderPage` e bad.
class PrintPage extends AsyncStatelessComponent {
  /// [printKinds] er ekta.
  final String kind;

  /// URL query (`token`, `paper`).
  final Map<String, String> query;

  /// PrintPage.
  const PrintPage(this.kind, this.query, {super.key});

  @override
  Future<Component> build(BuildContext context) async {
    final site = Site.of(context);
    final token = query['token'] ?? '';
    if (!_printToken.hasMatch(token)) {
      context.setStatusCode(404);
      return const StatusPage(StatusKind.notFound);
    }
    final PrintData data;
    try {
      data = await loadPrint(site.catalog.api, kind, token);
    } catch (e) {
      return pageError(context, e);
    }
    // Ochena paper A4: typo te print atke na.
    final thermal = query['paper'] == '80mm';
    final title = site.t('print.$kind');
    return div(classes: thermal ? 'paper-80mm' : 'paper-a4', [
      SeoHead(title: title, noindex: true),
      // @page class diye scope hoy na, tai page-i dey (CSP style-src 'unsafe-inline').
      Document.head(
        children: [
          Component.element(
            tag: 'style',
            attributes: const {'data-print': ''},
            children: [RawText(thermal ? '@page{size:80mm auto;margin:3mm}' : '@page{size:A4;margin:12mm}')],
          ),
        ],
      ),
      p(classes: 'notice mb-4 max-w-none print:hidden', [.text(site.t('print.hint'))]),
      ...switch (kind) {
        'packing_slip' => [for (final s in data.slips) _slip(site, title, s)],
        'invoice' => [for (final v in data.invoices) _invoice(site, title, v)],
        _ => [_pickList(site, title, data.pickList!)],
      },
    ]);
  }
}

// Ek order/list ek kagoje.
Component _sheet(List<Component> children) =>
    section(classes: 'print-sheet space-y-3 break-after-page last:break-after-auto bg-surface p-4 print:p-0', children);

Component _shopHead(Site site, String title, String number) => div(classes: 'flex justify-between gap-4', [
  div([
    h1(classes: 'text-2xl', [.text(title)]),
    p(classes: 'font-heading text-xl', [.text(number)]),
  ]),
  div(classes: 'text-right text-sm', [
    p([.text(site.settings.name.of(site.locale))]),
    if (site.settings.phone case final ph?) p([.text(ph)]),
  ]),
]);

String _label(Site site, PrintItem x) {
  final v = x.variantLabel.of(site.locale);
  return v.isEmpty ? x.productName.of(site.locale) : '${x.productName.of(site.locale)} ($v)';
}

String? _weight(Site site, int? g) => g == null ? null : site.t('print.grams', {'n': site.number(g)});

Component _cell(String s, {bool end = false}) => td(classes: 'border-b border-line py-1 ${end ? 'text-right' : ''}', [
  .text(s),
]);

Component _head(List<String> cols, {int endFrom = 99}) => thead([
  tr([
    for (final (i, c) in cols.indexed)
      th(classes: 'border-b border-ink py-1 text-left font-heading ${i >= endFrom ? 'text-right' : ''}', [.text(c)]),
  ]),
]);

Component _slip(Site site, String title, Slip s) => _sheet([
  _shopHead(site, title, '${s.orderNumber}-${s.seq}'),
  dl([
    row(site.t('order.recipient'), '${s.recipientName}, ${s.recipientPhone}'),
    row(site.t('order.address'), '${s.address}, ${s.area}, ${s.district}'),
    if (s.note case final n? when n.isNotEmpty) row(site.t('print.note'), n),
    row(site.t('order.payment'), site.t('order.payment.${s.paymentMethod}')),
    rowOf(
      site.t('print.cod'),
      span(classes: 'font-heading text-xl', [
        .text(s.codAmount > 0 ? site.taka(s.codAmount) : site.t('print.prepaid')),
      ]),
    ),
  ]),
  table(classes: 'w-full text-sm', [
    _head([site.t('order.items'), site.t('print.sku'), site.t('print.weight'), site.t('print.qty')], endFrom: 2),
    tbody([
      for (final x in s.items)
        tr([
          _cell(_label(site, x)),
          _cell(x.sku),
          _cell(_weight(site, x.weightG) ?? '', end: true),
          _cell(site.number(x.quantity), end: true),
        ]),
    ]),
  ]),
]);

Component _invoice(Site site, String title, Invoice v) => _sheet([
  _shopHead(site, title, v.orderNumber),
  p(classes: 'text-sm', [
    .text(site.t('order.placed_at', {'date': site.date(v.placedAt)})),
  ]),
  dl([
    row(site.t('order.customer'), '${v.customerName}, ${v.customerPhone}'),
    // Ekoi hole abar na.
    if (v.recipientName != v.customerName || v.recipientPhone != v.customerPhone)
      row(site.t('order.recipient'), '${v.recipientName}, ${v.recipientPhone}'),
    row(site.t('order.address'), '${v.address}, ${v.area}, ${v.district}'),
    row(site.t('order.payment'), site.t('order.payment.${v.paymentMethod}')),
    row(site.t('print.payment_status'), site.t('print.payment_status.${v.paymentStatus}')),
  ]),
  table(classes: 'w-full text-sm', [
    _head([
      site.t('order.items'),
      site.t('print.qty'),
      site.t('print.unit_price'),
      site.t('print.line_total'),
    ], endFrom: 1),
    tbody([
      for (final x in v.items)
        tr([
          _cell([_label(site, x), ?_weight(site, x.weightG)].join(' · ')),
          _cell(site.number(x.quantity), end: true),
          _cell(site.taka(x.unitPrice ?? 0), end: true),
          _cell(site.taka(x.lineTotal ?? 0), end: true),
        ]),
    ]),
  ]),
  dl(classes: 'ml-auto max-w-xs', [
    row(site.t('checkout.subtotal'), site.taka(v.subtotal)),
    if (v.discount > 0) row(site.t('checkout.discount'), site.taka(-v.discount)),
    row(site.t('checkout.delivery_charge'), site.taka(v.deliveryCharge)),
    rowOf(
      site.t('checkout.total'),
      span(classes: 'font-heading text-xl', [.text(site.taka(v.total))]),
      classes: 'border-t border-ink pt-1',
    ),
  ]),
]);

Component _pickTable(Site site, List<PickVariant> vs) => table(classes: 'w-full text-sm', [
  _head([site.t('order.items'), site.t('print.sku'), site.t('print.weight'), site.t('print.qty')], endFrom: 2),
  tbody([
    for (final x in vs)
      tr([
        _cell(
          x.variantLabel.of(site.locale).isEmpty
              ? x.productName.of(site.locale)
              : '${x.productName.of(site.locale)} (${x.variantLabel.of(site.locale)})',
        ),
        _cell(x.sku),
        _cell(_weight(site, x.orderedWeightG) ?? '', end: true),
        _cell(site.number(x.quantity), end: true),
      ]),
  ]),
]);

Component _pickList(Site site, String title, PickList l) => _sheet([
  // Tarikh shop er din (token e), format na: ISO-i kagoje sobcheye spashto.
  _shopHead(site, title, l.date),
  if (l.fulfillments.isEmpty)
    p([.text(site.t('print.empty'))])
  else ...[
    h2(classes: 'text-xl', [.text(site.t('print.total'))]),
    _pickTable(site, l.variants),
    for (final a in l.areas) ...[
      h2(classes: 'text-xl', [.text('${a.area}, ${a.district}')]),
      _pickTable(site, a.variants),
    ],
    h2(classes: 'text-xl', [.text(site.t('print.orders'))]),
    ul(classes: 'text-sm', [
      for (final f in l.fulfillments)
        li(classes: 'border-b border-line py-1', [
          .text('${f.orderNumber}-${f.seq} · ${f.recipientName} · ${f.area}, ${f.district}'),
        ]),
    ]),
  ],
]);
