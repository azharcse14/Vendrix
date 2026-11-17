import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';

import '../auth/auth_bloc.dart';
import '../l10n/locale_cubit.dart';
import '../permissions.dart';

/// Menu er route (ekhon je screen ache), kram e; lekha `admin.nav.<naam>`.
const menuRoutes = [
  '/',
  '/orders',
  '/packing',
  '/shipments',
  '/products',
  '/categories',
  '/inventory',
  '/staff',
];

const _icons = {
  '/': Icons.dashboard_outlined,
  '/orders': Icons.receipt_long_outlined,
  '/packing': Icons.inventory_2_outlined,
  '/shipments': Icons.local_shipping_outlined,
  '/products': Icons.shopping_basket_outlined,
  '/categories': Icons.account_tree_outlined,
  '/inventory': Icons.warehouse_outlined,
  '/staff': Icons.badge_outlined,
};

String _label(BuildContext context, String r) =>
    context.t('admin.nav.${r == '/' ? 'dashboard' : r.substring(1)}');

/// Login er porer page er gora: upore naam, bhasha, logout; menu role onujayi (`canOpen`, 11.2),
/// 600px er niche drawer, upore rail (11.5).
class AppShell extends StatelessWidget {
  final String location;
  final Widget child;
  const AppShell({super.key, required this.location, required this.child});

  @override
  Widget build(BuildContext context) {
    final auth = context.watch<AuthBloc>().state;
    final routes = auth is Authenticated
        ? menuRoutes.where(auth.staff.canOpen).toList()
        : <String>[];
    // `/orders/5` → `/orders`
    final current = routes.lastIndexWhere(
      (r) => r == '/' ? location == '/' : location.startsWith(r),
    );
    final wide = MediaQuery.sizeOf(context).width >= 600;
    return Scaffold(
      appBar: AppBar(
        title: Text(
          current < 0
              ? context.t('admin.title')
              : _label(context, routes[current]),
        ),
        actions: [
          TextButton(
            onPressed: context.read<LocaleCubit>().toggle,
            child: Text(context.t('admin.language')),
          ),
          IconButton(
            tooltip: context.t('admin.logout'),
            onPressed: () => context.read<AuthBloc>().add(LogoutRequested()),
            icon: const Icon(Icons.logout),
          ),
        ],
      ),
      drawer: wide
          ? null
          : Drawer(
              child: ListView(
                children: [
                  for (final (i, r) in routes.indexed)
                    ListTile(
                      key: Key('nav.$r'),
                      leading: Icon(_icons[r]),
                      title: Text(_label(context, r)),
                      selected: i == current,
                      onTap: () {
                        Navigator.of(context).pop();
                        context.go(r);
                      },
                    ),
                ],
              ),
            ),
      body: wide
          ? Row(
              children: [
                NavigationRail(
                  selectedIndex: current < 0 ? null : current,
                  labelType: NavigationRailLabelType.all,
                  onDestinationSelected: (i) => context.go(routes[i]),
                  destinations: [
                    for (final r in routes)
                      NavigationRailDestination(
                        icon: Icon(_icons[r], key: Key('nav.$r')),
                        label: Text(_label(context, r)),
                      ),
                  ],
                ),
                const VerticalDivider(width: 1),
                Expanded(child: child),
              ],
            )
          : child,
    );
  }
}
