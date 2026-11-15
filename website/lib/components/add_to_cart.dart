import 'package:http/http.dart' as http;
import 'package:jaspr/jaspr.dart';
import 'package:shared/api/api_client.dart';
import 'package:universal_web/web.dart' as web;

/// Product page er `form#cart-add` (10.12 item 4): JS chhara `POST /_/cart/items` → `303 /cart`; JS e JSON e BFF,
/// uttor er `count` badge e (`[data-cart-badge]`), `#cart-add-status` dekhay, error `#cart-add-error` e (lekha
/// server er `[data-cart-error]`). `[data-qty-step]` button (JS chhara lukano) quantity bodlay. Dam client e na.
@client
class AddToCart extends StatefulComponent {
  /// `0`–`9` shop er digit e (badge; client e intl na, JS choto).
  final String digits;

  /// AddToCart.
  const AddToCart({required this.digits, super.key});

  @override
  State<AddToCart> createState() => _AddToCartState();
}

class _AddToCartState extends State<AddToCart> {
  bool _busy = false;
  late final ApiClient _bff = ApiClient(Uri.base, http.Client());

  @override
  void initState() {
    super.initState();
    if (!kIsWeb) return;
    final form = web.document.getElementById('cart-add') as web.HTMLFormElement?;
    final qty = form?.querySelector('input[name="quantity"]') as web.HTMLInputElement?;
    if (form == null || qty == null) return;
    final steps = form.querySelectorAll('[data-qty-step]');
    for (var i = 0; i < steps.length; i++) {
      final b = steps.item(i) as web.Element;
      b.removeAttribute('hidden');
      web.EventStreamProviders.clickEvent.forTarget(b).listen((_) {
        final max = int.tryParse(qty.max) ?? 50;
        final n = (int.tryParse(qty.value) ?? 1) + (int.tryParse(b.getAttribute('data-qty-step') ?? '') ?? 0);
        qty.value = '${n.clamp(1, max)}';
      });
    }
    web.EventStreamProviders.submitEvent.forTarget(form).listen((e) {
      e.preventDefault();
      _add(form, qty);
    });
  }

  Future<void> _add(web.HTMLFormElement form, web.HTMLInputElement qty) async {
    if (_busy) return;
    _busy = true;
    final variant =
        (form.querySelector('input[name="variant_id"]:checked') ??
                form.querySelector('input[type="hidden"][name="variant_id"]'))
            as web.HTMLInputElement?;
    final status = web.document.getElementById('cart-add-status');
    status?.setAttribute('hidden', '');
    _error(null);
    try {
      final res = await _bff.send(
        'POST',
        '/_/cart/items',
        body: {'variant_id': int.tryParse(variant?.value ?? ''), 'quantity': int.tryParse(qty.value)},
      );
      if (res case {'count': final int count}) _badge(count);
      status?.removeAttribute('hidden');
    } on ApiError catch (e) {
      _error(e.code);
    } finally {
      _busy = false;
    }
  }

  void _badge(int count) {
    final text = [for (final c in '$count'.codeUnits) component.digits[c - 0x30]].join();
    final badges = web.document.querySelectorAll('[data-cart-badge]');
    for (var i = 0; i < badges.length; i++) {
      final b = badges.item(i) as web.Element;
      b.textContent = text;
      b.toggleAttribute('hidden', count == 0);
    }
  }

  void _error(String? code) {
    final box = web.document.getElementById('cart-add-error');
    if (box == null) return;
    // Code selector e jay, tai shudhu Go er dhoron (`[a-z_]`).
    final known = code != null && RegExp(r'^[a-z_]+$').hasMatch(code) ? code : 'internal';
    final text = code == null
        ? null
        : (web.document.querySelector('[data-cart-error="$known"]') ??
                  web.document.querySelector('[data-cart-error="internal"]'))
              ?.textContent;
    box.textContent = text ?? '';
    box.toggleAttribute('hidden', text == null);
  }

  @override
  Component build(BuildContext context) => const Component.empty();
}
