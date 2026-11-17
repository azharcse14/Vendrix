import 'dart:convert';

import 'package:html/dom.dart' as dom;
import 'package:html/parser.dart' show parse;
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:jaspr/server.dart';
import 'package:shared/shared.dart';
import 'package:shelf/shelf.dart' show Pipeline;
import 'package:test/test.dart';
import 'package:website/main.server.options.dart';
import 'package:website/services/cache_control.dart';
import 'package:website/services/config.dart';
import 'package:website/services/render.dart';
import 'package:website/services/security_headers.dart';
import 'package:website/services/settings_cache.dart';

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  final cfg = Config.fromEnv({
    'API_BASE_URL': 'http://api.test',
    'JASPR_INTERNAL_SECRET': 's',
    'IMAGE_BASE_URL': 'https://img.test',
    'IMAGE_RESIZE': 'none',
    'TURNSTILE_SITE_KEY': '1x00000000000000000000AA',
  });
  const t = Translator('bn');
  const tok = 'eyJzaG9wX2lkIjoxfQ.c2lnbmF0dXJl';
  const json = {'content-type': 'application/json; charset=utf-8'};
  String taka(int p) => formatTaka(p, 'bn');

  Map<String, dynamic> item({int? unit, int? line, bool weight = false}) => {
    'product_name': {'bn': 'লাল চাল', 'en': 'Red rice'},
    'variant_label': {'bn': '৫ কেজি', 'en': '5 kg'},
    'sku': 'R8',
    'quantity': 2,
    'variable_weight': weight,
    'ordered_weight_g': weight ? 1500 : null,
    'packed_weight_g': null,
    'unit_price': ?unit,
    'line_total': ?line,
  };
  final slip = {
    'order_number': 'VX-100001',
    'seq': 1,
    'vendor_id': 1,
    'recipient_name': 'করিম',
    'recipient_phone': '+8801812345678',
    'district': 'Dhaka',
    'area': 'Mirpur',
    'address': 'বাড়ি ১২, রোড ৩',
    'note': 'বিকেলে দিন',
    'payment_method': 'cod',
    'cod_amount': 176000,
    'items': [item()],
  };
  final invoice = {
    'order_number': 'VX-100001',
    'placed_at': '2026-11-16T04:00:00Z',
    'customer_name': 'রহিম',
    'customer_phone': '+8801712345678',
    'recipient_name': 'করিম',
    'recipient_phone': '+8801812345678',
    'district': 'Dhaka',
    'area': 'Mirpur',
    'address': 'বাড়ি ১২, রোড ৩',
    'payment_method': 'cod',
    'payment_status': 'unpaid',
    'subtotal': 170000,
    'discount': 5000,
    'delivery_charge': 6000,
    'total': 171000,
    'items': [item(unit: 85000, line: 170000)],
  };
  Map<String, dynamic> pickVariant(int qty) => {
    'variant_id': 8,
    'product_name': {'bn': 'লাল চাল', 'en': 'Red rice'},
    'variant_label': {'bn': '৫ কেজি', 'en': '5 kg'},
    'sku': 'R8',
    'variable_weight': false,
    'quantity': qty,
    'ordered_weight_g': null,
  };
  var pickEmpty = false;
  final api = MockClient((r) async {
    final q = r.url.queryParameters;
    final kind = q['kind'];
    Object? body;
    if (r.url.path == '/api/print' && q['token'] == tok) {
      body = switch (kind) {
        'packing_slip' => {
          'kind': kind,
          'slips': [slip],
        },
        'invoice' => {
          'kind': kind,
          'invoices': [invoice],
        },
        'pick_list' => {
          'kind': kind,
          'pick_list': {
            'date': '2026-11-17',
            'variants': pickEmpty ? [] : [pickVariant(7)],
            'areas': pickEmpty
                ? []
                : [
                    {
                      'district': 'Dhaka',
                      'area': 'Mirpur',
                      'variants': [pickVariant(7)],
                    },
                  ],
            'fulfillments': pickEmpty
                ? []
                : [
                    {
                      'id': 11,
                      'order_id': 5,
                      'order_number': 'VX-100001',
                      'seq': 1,
                      'vendor_id': 1,
                      'recipient_name': 'করিম',
                      'district': 'Dhaka',
                      'area': 'Mirpur',
                      'confirmed_at': '2026-11-17T03:00:00Z',
                    },
                  ],
          },
        },
        _ => null,
      };
    }
    if (body == null) return http.Response('{"error":{"code":"not_found","message":""}}', 404, headers: json);
    return http.Response(jsonEncode(body), 200, headers: json);
  });

  Future<ShopSettings> shop(String _) async => const ShopSettings(
    PublicSettings({
      'branding': {
        'name': {'bn': 'সবুজ বাজার', 'en': 'Sobuj Bazar'},
        'phone': '01700000000',
      },
    }),
    1,
    {},
  );

  Future<(Response, String, dom.Document)> page(String path) async {
    final handler = const Pipeline()
        .addMiddleware(securityHeaders(Uri.parse('https://img.test')))
        .addMiddleware(cacheControl())
        .addHandler(serveApp((r, render) => renderPage(r, render, shop, cfg: cfg, client: api, log: (_) {})));
    final res = await handler(Request('GET', Uri.parse('http://shop.test$path'), headers: {'host': 'shop.test'}));
    final html = await res.readAsString();
    return (res, html, parse(html));
  }

  // Token page header, script na (client JS na, 10.1 v1.5), site er header/footer/nav na.
  void printPage(Response res, dom.Document doc, String path) {
    expect(res.statusCode, 200, reason: path);
    expect(res.headers['cache-control'], 'private, no-store', reason: path);
    expect(res.headers['referrer-policy'], 'no-referrer', reason: path);
    expect(res.headers['x-robots-tag'], 'noindex', reason: path);
    expect(doc.querySelector('meta[name="robots"]')!.attributes['content'], 'noindex', reason: path);
    expect(doc.querySelectorAll('script'), isEmpty, reason: path);
    expect(doc.querySelector('body > header, footer, nav'), isNull, reason: path);
    // Ctrl+P er kotha screen e, print e na.
    expect(doc.querySelector('.print\\:hidden')!.text, t.t('print.hint'), reason: path);
  }

  group('Print page /print/{kind}?token= (10.1, 11.6, 11.7 item 8)', () {
    setUp(() => pickEmpty = false);

    test('packing slip: number-seq, recipient, thikana, note, COD taka, item, SKU, qty; dam na; A4', () async {
      final (res, html, doc) = await page('/print/packing_slip?token=$tok');
      printPage(res, doc, 'packing_slip');
      final text = doc.body!.text;
      expect(doc.querySelector('h1')!.text, t.t('print.packing_slip'));
      for (final s in [
        'সবুজ বাজার',
        '01700000000',
        'VX-100001-1',
        'করিম',
        '+8801812345678',
        'বাড়ি ১২, রোড ৩',
        'Mirpur',
        'Dhaka',
        'বিকেলে দিন',
        t.t('order.payment.cod'),
        taka(176000),
        'লাল চাল',
        '৫ কেজি',
        'R8',
      ]) {
        expect(text, contains(s));
      }
      // Packing slip e item er dam na.
      expect(text, isNot(contains(taka(170000))));
      expect(doc.querySelectorAll('.print-sheet'), hasLength(1));
      expect(doc.querySelector('.paper-a4'), isNotNull);
      expect(doc.querySelector('head style[data-print]')!.text, contains('A4'));
    });

    test('?paper=80mm: class ar @page 80mm; ochena paper A4', () async {
      final (res, _, doc) = await page('/print/packing_slip?token=$tok&paper=80mm');
      printPage(res, doc, '80mm');
      expect(doc.querySelector('.paper-80mm'), isNotNull);
      expect(doc.querySelector('.paper-a4'), isNull);
      expect(doc.querySelector('head style[data-print]')!.text, contains('80mm'));
      final (_, _, doc2) = await page('/print/packing_slip?token=$tok&paper=xx');
      expect(doc2.querySelector('.paper-a4'), isNotNull);
    });

    test('invoice: customer, recipient, item dam, subtotal, chhar, delivery, total, payment status', () async {
      final (res, _, doc) = await page('/print/invoice?token=$tok');
      printPage(res, doc, 'invoice');
      final text = doc.body!.text;
      expect(doc.querySelector('h1')!.text, t.t('print.invoice'));
      for (final s in [
        'VX-100001',
        'রহিম',
        '+8801712345678',
        'করিম',
        taka(85000),
        taka(170000),
        taka(-5000),
        taka(6000),
        taka(171000),
        t.t('print.payment_status.unpaid'),
      ]) {
        expect(text, contains(s));
      }
    });

    test('pick list: tarikh, mot variant, area onujayi, fulfillment; khali hole lekha', () async {
      final (res, _, doc) = await page('/print/pick_list?token=$tok');
      printPage(res, doc, 'pick_list');
      final text = doc.body!.text;
      expect(doc.querySelector('h1')!.text, t.t('print.pick_list'));
      for (final s in ['2026-11-17', 'লাল চাল', 'R8', formatCount(7, 'bn'), 'Mirpur', 'VX-100001-1', 'করিম']) {
        expect(text, contains(s));
      }
      pickEmpty = true;
      final (res2, _, doc2) = await page('/print/pick_list?token=$tok');
      expect(res2.statusCode, 200);
      expect(doc2.body!.text, contains(t.t('print.empty')));
    });

    test('English: /en/print nai (404); lekha {bn, en} theke bn', () async {
      final (res, _, _) = await page('/en/print/invoice?token=$tok');
      expect(res.statusCode, 404);
    });

    test('bhul/expire token, onno kind, token nai, ochena kind: 404, no-store', () async {
      for (final path in [
        '/print/invoice?token=bad',
        '/print/invoice',
        '/print/other?token=$tok',
        '/print/invoice?token=a%20b',
      ]) {
        final (res, _, doc) = await page(path);
        expect(res.statusCode, 404, reason: path);
        expect(res.headers['cache-control'], 'private, no-store', reason: path);
        expect(res.headers['referrer-policy'], 'no-referrer', reason: path);
        expect(doc.querySelectorAll('script'), isEmpty, reason: path);
      }
    });
  });
}
