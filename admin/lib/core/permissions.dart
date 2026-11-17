import 'package:shared/shared.dart';

const _all = {'owner', 'admin', 'staff', 'vendor_owner', 'vendor_staff'};
const _ownerAdmin = {'owner', 'admin'};

// 11.2 er table, route er prothom bhag → role. Lomba prefix age (`/settings/credentials`).
const _routes = <String, Set<String>>{
  '/settings/credentials': {'owner'},
  '/settings': _ownerAdmin,
  '': _all, // dashboard
  '/orders': _all,
  '/packing': _all,
  '/shipments': _all,
  '/inventory': _all, // staff: stock entry ar wastage
  '/products': {'owner', 'admin', 'vendor_owner'},
  '/categories': {'owner', 'admin', 'vendor_owner'},
  '/coupons': {'owner', 'admin', 'vendor_owner'},
  '/reviews': _ownerAdmin,
  '/customers': _ownerAdmin,
  '/zones': _ownerAdmin,
  '/vendors': {'owner', 'admin', 'vendor_owner'},
  '/staff': {'owner', 'vendor_owner'},
  '/audit': _ownerAdmin,
  '/money': {'owner', 'admin', 'vendor_owner'},
  '/complaints': _ownerAdmin,
  '/privacy': _ownerAdmin,
  '/reports': _ownerAdmin,
};

/// Role onujayi route (11.2). Shudhu menu/route lukano; suraksha backend e.
extension StaffPermissions on StaffProfile {
  /// [location] (`/orders/12`) ei role khulte pare kina; ochena route e false.
  bool canOpen(String location) {
    for (final e in _routes.entries) {
      final p = e.key;
      final match = p.isEmpty
          ? location == '/'
          : location == p || location.startsWith('$p/');
      if (match) return e.value.contains(role);
    }
    return false;
  }
}
