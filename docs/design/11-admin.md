> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 11. Admin panel (Flutter web)

### 11.1 Module ar route

| Route | Module |
|---|---|
| `/login`, `/login/totp`, `/login/setup` | Login, prothom login e password ar TOTP setup |
| `/` | Dashboard |
| `/orders`, `/orders/:id` | Order (filter: status, tarikh, phone, risk flag; "confirm er opekkhay" tab) |
| `/packing` | Pick list, packing (ojon entry), print |
| `/shipments` | Delivery, rider assign, tracking |
| `/products`, `/products/new`, `/products/:id` | Product, variant, chobi, SEO, attributes |
| `/categories` | Category tree |
| `/inventory`, `/inventory/:variantId` | Batch, stock, movement, alert |
| `/coupons` | Coupon |
| `/customers`, `/customers/:id` | Customer, block |
| `/reviews` | Review moderation |
| `/zones` | Delivery zone |
| `/money/cod`, `/money/payments`, `/money/refunds` | COD settlement, gateway settlement, refund |
| `/settings/branding`, `/settings/theme`, `/settings/language`, `/settings/translations`, `/settings/homepage`, `/settings/checkout`, `/settings/features`, `/settings/credentials` | Store settings |
| `/staff`, `/audit` | Staff, audit log |
| `/vendors`, `/vendors/:id` | Vendor (marketplace; single store e shudhu house vendor er profile) |
| `/complaints`, `/privacy` | Support |
| `/reports` | Report (Phase 2 bistarito) |

### 11.2 Role ar onumoti

| Module | owner | admin | staff | vendor_owner | vendor_staff |
|---|---|---|---|---|---|
| Dashboard, order dekha, packing, shipment | Hya | Hya | Hya | Nijer fulfillment | Nijer fulfillment |
| Order status change, edit | Hya | Hya | Hya (cancel chhara; v1.5: je edit e kono vendor er shob item bad ba refund hoy, sheta na; v1.7: fulfillment cancel na, ar `returned` shudhu refund chhara, jemon COD e taka newa hoyni) | Nijer fulfillment: pack, ship, cancel (pack er age) | Pack, ship |
| Order cancel, refund | Hya | Hya | Na | Na | Na |
| Product, category, inventory | Hya | Hya | Stock entry ar wastage | Nijer product ar stock (category na) | Stock entry ar wastage |
| Coupon, review, customer block | Hya | Hya | Na | Nijer vendor er coupon (Phase 3) | Na |
| Delivery zone | Hya | Hya | Na | Na | Na |
| Store settings (theme, bhasha, homepage, checkout) | Hya | Hya | Na | Na | Na |
| Vendor (jog, suspend, commission) | Hya | Hya | Na | Nijer profile | Na |
| Credential (payment/SMS/courier secret) | Hya | Na | Na | Na | Na |
| Staff, 2FA reset | Hya | Na | Na | Nijer vendor er staff | Na |
| Audit log | Hya | Hya | Na | Na | Na |
| Taka (COD, settlement) | Hya | Hya | Na | Nijer earning, payout (Phase 3) | Na |

Vendor role gulo shudhu `shops.mode = 'marketplace'` e. Backend protiti admin endpoint e role ar vendor scope check kore. Flutter e role check shudhu UI lukanor jonno, suraksha na.

### 11.3 Code er niyom

1. `get_it`: `di.dart` e shob register (`ApiClient`, repository). `getIt<...>()` shudhu `di.dart` ar `BlocProvider(create: ...)` e.
2. `bloc`: shohoj e `Cubit` (theme, bhasha, detail page), onek event e `Bloc` (order list filter/paging, checkout-er moto form). State `sealed class`. App-wide: `AuthBloc`, `ThemeCubit`, `LocaleCubit`.
3. `go_router`: `usePathUrlStrategy()`. `redirect` e setup, login ar role check, `refreshListenable` e `AuthBloc` er stream. Router `buildRouter(AuthBloc)` factory; `getIt` shudhu `di.dart` e.

```dart
// di.dart e: getIt.registerSingleton<GoRouter>(buildRouter(getIt<AuthBloc>()));
GoRouter buildRouter(AuthBloc auth) => GoRouter(
  refreshListenable: GoRouterRefreshStream(auth.stream),
  redirect: (context, state) {
    final s = auth.state;
    final loc = state.matchedLocation;
    if (s is SetupRequired) return loc == '/login/setup' ? null : '/login/setup';
    if (s is! Authenticated) return loc.startsWith('/login') ? null : '/login';
    if (loc.startsWith('/login')) return '/';
    if (!s.staff.canOpen(loc)) return '/';
    return null;
  },
  routes: [ /* 11.1 er route */ ],
);
```

`GoRouterRefreshStream` ekta choto `ChangeNotifier` class, stream e notun value ashle `notifyListeners()` dake (nije likhte hobe, 10 line).

4. **Token**: admin er session cookie `__Host-asid` (`HttpOnly; Secure; SameSite=Strict; Path=/`, api domain e). `di.dart` e `ApiClient` ke `BrowserClient()..withCredentials = true` dao (shared package pure Dart thake, client inject hoy). CORS e `Access-Control-Allow-Credentials` shudhu `ADMIN_ORIGINS` er jonno. Dart code token dekhe na, localStorage e kichu na.

### 11.4 Guruttopurno screen

**Order list**: status tab, "confirm er opekkhay" ar "deadline kache" alada highlight, risk flag chip, phone search. Ek click e call (phone link), confirm, cancel.

**Order detail**: item (snapshot), total er bhag, status history, allowed transition er button (backend er `CanTransitionOrder`/`CanTransitionFulfillment` er moto; marketplace e fulfillment onujayi bhag kore), note, shipment, refund, print. Online unpaid order e "Confirm" button nai (`payment_required`).

**Packing**: ajker pick list, order dhore pack, ojon-variable item e asol ojon input (gram), shob pack hole "Packed". Batch automatic (FIFO), staff dekhte pay kon batch.

**Theme editor**: bame preset, 5 ta color picker, font dropdown (Bangla sample lekha shoho), radius slider; dane live preview (product card, header, button) **mobile/desktop toggle** shoho; contrast warning shathe shathe; "Draft save", "Preview website e", "Publish", "History/rollback".

**Translation editor**: key ba lekha diye search, Bangla ar English pashapashi, default dekhano, "Default e ferot".

### 11.5 Mobile e admin

Packing staff phone diye kaj korbe. Order list, order detail, packing ar shipment screen 360px e kaj korbe (list view, boro button). Baki screen desktop-first holeo cholbe.

### 11.6 Print

Flutter web e print kothin, tai admin `POST /api/admin/print-links {kind, order_ids}` diye 10 minute er signed link nay, ar notun tab e website er `/print/{kind}?token=` khole (packing slip, pick list, invoice). Website er print CSS A4 ar 80mm thermal duitar jonno.
