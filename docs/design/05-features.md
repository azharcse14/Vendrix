> ESD design er ongsho · [Suchipotro ar version](README.md) · Shob kaj e age: Section 1.4 ([01-overview.md](01-overview.md))

## 5. Feature list (phase onujayi)

**MVP** = limited launch, **2** = public launch, **3** = growth.

### 5.1 Customer website

| Bhag | Feature | Phase |
|---|---|---|
| Account | Phone + OTP login/register | MVP |
| Account | Profile, address book, bhasha pochhondo | MVP |
| Account | Account delete ar data download request | MVP |
| Account | Google/Facebook login | 2 |
| Khuja | Home: banner, category, featured product | MVP |
| Khuja | Category page: filter (dam, stock), sort | MVP |
| Khuja | Search: Bangla, English, Banglish keyword | MVP |
| Khuja | Search auto-suggest | 2 |
| Khuja | Recommendation | 3 |
| Product | Gallery (swipe), variant, stock, review, wishlist, share | MVP |
| Product | Organic tottho: ojon onujayi dam, utpadoner jayga, harvest/expiry, certification, BFSA number | MVP |
| Product | "Stock ashle janao" | 2 |
| Product | Seasonal pre-order (advance shudhu 10% porjonto ba escrow, 12.1), combo pack | 2 |
| Cart | Guest cart, login e merge | MVP |
| Checkout | Login chhara order (guest checkout) | MVP |
| Checkout | Onner thikanay pathano (recipient alada) | MVP |
| Checkout | Coupon, area onujayi delivery charge, free delivery shima, order note | MVP |
| Checkout | Delivery area check, same-day cutoff time | MVP |
| Checkout | Jhuki thakle OTP, bot protection (Turnstile + honeypot) | MVP |
| Checkout | Cash on Delivery, SSLCommerz | MVP |
| Checkout | Privacy/terms consent, delivery er shomoy dekhano (perishable hole aro kom) | MVP |
| Checkout | bKash/Nagad direct, delivery time slot | 2 |
| Order | History, status, ship er age cancel, invoice (printable page) | MVP |
| Order | Login chhara tracking (order number + phone), SMS link (`public_token`) | MVP |
| Order | COD confirm link | MVP |
| Order | Courier live tracking, return request (chobi shoho), "abar order" | 2 |
| Review | Login ba SMS link diye, shudhu delivered order er product | MVP |
| Notification | SMS: order, confirm request, ship, deliver + review link | MVP |
| Notification | Push (PWA/app), abandoned cart reminder | 2 |
| Bhasha | Bangla/English, Bangla shonkhya, lakh-style taka | MVP |
| Design | Mobile-first, bottom nav, sticky cart bar, theme token | MVP |
| Design | PWA | 2 |
| Support | Policy page (Bangla), contact, FAQ, complaint form, DBID ar BFSA number | MVP |
| SEO | Title, description, OG, JSON-LD, sitemap, hreflang, canonical | MVP |
| Content | Recipe/blog | 2 |
| AI | Shopping assistant | 3 |
| Subscription | Saptahik bazar basket | 3 |

### 5.2 Admin panel

| Bhag | Feature | Phase |
|---|---|---|
| Dashboard | Ajker order, bikri, pending, low stock, expiry kache | MVP |
| Alert | Notun order (email/SMS), delivery deadline, low stock, expiry | MVP |
| Product | Product/variant/chobi/category, Bangla + English, SEO, search keyword, BFSA number, certificate upload | MVP |
| Product | Unit (kg/gram/litre/piece), ojon-variable product | MVP |
| Product | CSV diye bulk upload | 2 |
| Inventory | Batch (harvest, expiry, kena dam), stock movement itihash, wastage entry | MVP |
| Inventory | Supplier/krishok ar purchase entry | 2 |
| Order | List, filter, status change, cancel, refund mark, edit (item bad ba quantity kom, partial refund) | MVP |
| Order | Confirm er opekkhay order er list, phone call note (`order_notes`) | MVP |
| Packing | Pick list, packing slip (A4 + 80mm thermal), packing e asol ojon entry | MVP |
| Delivery | Delivery zone, charge, cutoff, nijer rider assign, courier tracking ID manual | MVP |
| Delivery | Courier API booking + webhook | 2 |
| Taka | COD settlement mela, SSLCommerz settlement mela | MVP |
| Customer | List, order history, phone block | MVP |
| Customer | COD fraud score (courier data) | 2 |
| Marketing | Coupon | MVP |
| Marketing | Facebook Pixel, GA4, flash sale (3), loyalty (3, Bangladesh Bank er niyom dekhe) | 2/3 |
| Review | Hide/unhide | MVP |
| Settings | Branding, theme (preset, rong, font, radius), live preview (mobile/desktop), contrast warning, draft/preview/publish/rollback | MVP |
| Settings | Bhasha, translation override, homepage, checkout setting (OTP mode, risk niyom), feature module on/off | MVP |
| Staff | Role (owner/admin/staff), 2FA (recovery code shoho), prothom login e password ar TOTP setup, audit log | MVP |
| Report | Bistarito sales, lav (kena dam theke), CSV export | 2 |
| Support | Complaint log | MVP |

### 5.3 Business module (config diye on/off)

| Module key | Ki | Kar jonno | Phase |
|---|---|---|---|
| `unit_pricing` | Ojon/unit onujayi dam ("/ কেজি") | Organic | MVP |
| `variable_weight` | Packing e asol ojon diye final dam | Organic (mach, mangsho, shobji) | MVP |
| `batch_expiry` | Batch, harvest, expiry, FIFO | Organic | MVP |
| `certification` | Certificate badge ar document | Organic | MVP |
| `delivery_slot` | Delivery shomoy bachai | Organic | 2 |
| `preorder` | Mousumi pre-order | Organic | 2 |
| `size_chart` | Size chart | Fashion | Jokhon ashbe |
| `specs` | Specification table, compare | Electronics | Jokhon ashbe |
| `warranty` | Warranty tottho | Electronics | Jokhon ashbe |

### 5.4 Marketplace ar multi-tenant (Phase 3)

| Bhag | Feature | Mode |
|---|---|---|
| Vendor | Vendor invite/signup, KYC kagoj (trade license, DBID, food hole BFSA), approve, suspend | Marketplace |
| Vendor | Seller center (ekoi Flutter admin, vendor role): nijer product, stock, fulfillment, pack, ship, earning | Marketplace |
| Vendor | Vendor store page (`/v/{slug}`), product page e vendor er naam, vendor rating | Marketplace |
| Catalog | Vendor er product publish er age approval, "organic" dabi ar certificate check | Marketplace |
| Taka | Vendor onujayi commission, ledger, saptahik payout, return window porjonto hold | Marketplace |
| Checkout | Ek cart e onek vendor, vendor onujayi parcel, delivery charge `per_order` ba `per_vendor` | Marketplace |
| Platform | Platform admin: shop khola, suspend, usage | Multi-tenant |
| Platform | Custom domain (verify + auto HTTPS), shop onujayi SSLCommerz/SMS credential | Multi-tenant |
| Platform | Shop er plan ar billing | Multi-tenant |
