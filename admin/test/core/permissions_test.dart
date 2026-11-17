import 'package:admin/core/permissions.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared/shared.dart';

StaffProfile staff(String role) => StaffProfile.fromJson({
  'name': 'N',
  'email': 'n@example.com',
  'role': role,
  'vendor_id': null,
});

void main() {
  // 11.2 er table: route → je role khulte pare (UI lukano; suraksha backend e).
  const all = ['owner', 'admin', 'staff', 'vendor_owner', 'vendor_staff'];
  const table = <String, List<String>>{
    '/': all,
    '/orders': all,
    '/orders/12': all,
    '/packing': all,
    '/shipments': all,
    '/inventory': all,
    '/inventory/5': all,
    '/products': ['owner', 'admin', 'vendor_owner'],
    '/products/new': ['owner', 'admin', 'vendor_owner'],
    '/categories': ['owner', 'admin', 'vendor_owner'],
    '/coupons': ['owner', 'admin', 'vendor_owner'],
    '/reviews': ['owner', 'admin'],
    '/customers': ['owner', 'admin'],
    '/customers/3': ['owner', 'admin'],
    '/zones': ['owner', 'admin'],
    '/settings/theme': ['owner', 'admin'],
    '/settings/credentials': ['owner'],
    '/vendors': ['owner', 'admin', 'vendor_owner'],
    '/staff': ['owner', 'vendor_owner'],
    '/audit': ['owner', 'admin'],
    '/money/cod': ['owner', 'admin', 'vendor_owner'],
    '/complaints': ['owner', 'admin'],
    '/privacy': ['owner', 'admin'],
    '/reports': ['owner', 'admin'],
    '/nai': [],
    '/ordersx': [],
  };

  for (final e in table.entries) {
    test('canOpen ${e.key} (11.2)', () {
      for (final role in all) {
        expect(
          staff(role).canOpen(e.key),
          e.value.contains(role),
          reason: '$role ${e.key}',
        );
      }
    });
  }

  test('ochena role kichui khulte pare na', () {
    expect(staff('rider').canOpen('/'), isFalse);
  });
}
