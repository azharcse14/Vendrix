import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:shared/shared.dart' show InventoryVariant;

import 'core/auth/auth_bloc.dart';
import 'core/browser.dart';
import 'core/l10n/locale_cubit.dart';
import 'core/locator.dart';
import 'core/permissions.dart';
import 'core/widgets/app_shell.dart';
import 'features/audit/bloc/audit_cubit.dart';
import 'features/audit/pages/audit_page.dart';
import 'features/auth/pages/login_page.dart';
import 'features/coupons/bloc/coupons_cubit.dart';
import 'features/coupons/pages/coupons_page.dart';
import 'features/customers/bloc/customer_detail_cubit.dart';
import 'features/customers/bloc/customers_cubit.dart';
import 'features/customers/pages/customer_detail_page.dart';
import 'features/customers/pages/customers_page.dart';
import 'features/categories/bloc/categories_cubit.dart';
import 'features/categories/pages/categories_page.dart';
import 'features/inventory/bloc/inventory_list_cubit.dart';
import 'features/inventory/bloc/variant_stock_cubit.dart';
import 'features/inventory/pages/inventory_page.dart';
import 'features/inventory/pages/variant_stock_page.dart';
import 'features/products/bloc/product_form_cubit.dart';
import 'features/products/bloc/product_list_cubit.dart';
import 'features/products/pages/product_form_page.dart';
import 'features/products/pages/product_list_page.dart';
import 'features/auth/pages/setup_page.dart';
import 'features/auth/pages/totp_page.dart';
import 'features/dashboard/bloc/dashboard_cubit.dart';
import 'features/dashboard/pages/dashboard_page.dart';
import 'features/orders/bloc/order_detail_cubit.dart';
import 'features/orders/bloc/order_list_bloc.dart';
import 'features/orders/pages/order_detail_page.dart';
import 'features/orders/pages/order_list_page.dart';
import 'features/packing/bloc/pack_cubit.dart';
import 'features/packing/bloc/pick_list_cubit.dart';
import 'features/packing/pages/pack_page.dart';
import 'features/packing/pages/packing_page.dart';
import 'features/shipments/bloc/shipments_cubit.dart';
import 'features/shipments/pages/shipments_page.dart';
import 'features/staff/bloc/staff_cubit.dart';
import 'features/staff/pages/staff_page.dart';
import 'features/zones/bloc/zones_cubit.dart';
import 'features/zones/pages/zones_page.dart';

/// Stream e notun value ele router abar redirect dekhe (11.3).
class GoRouterRefreshStream extends ChangeNotifier {
  late final StreamSubscription<Object?> _sub;
  GoRouterRefreshStream(Stream<Object?> stream) {
    _sub = stream.listen((_) => notifyListeners());
  }

  @override
  void dispose() {
    _sub.cancel();
    super.dispose();
  }
}

/// Setup, login ar role er guard (11.3); null = jekhane ache sekhanei.
String? authRedirect(AuthState s, String loc) {
  if (s is SetupRequired) return loc == '/login/setup' ? null : '/login/setup';
  if (s is TotpRequired) return loc == '/login/totp' ? null : '/login/totp';
  if (s is! Authenticated) return loc == '/login' ? null : '/login';
  if (loc.startsWith('/login')) return '/';
  if (!s.staff.canOpen(loc)) return '/';
  return null;
}

int _id(GoRouterState s, String name) =>
    int.tryParse(s.pathParameters[name]!) ?? 0; // 0 → API 404

/// 11.1 er route (ekhon login, dashboard, order, packing, shipment, product, category, inventory, coupon, zone,
/// customer, staff, audit; settings M8 e, review M9 e, money M11 e). Screen er bloc
/// route e `BlocProvider(create:)` (getIt shudhu ekhane ar di.dart e, 11.3).
GoRouter buildRouter(AuthBloc auth) => GoRouter(
  refreshListenable: GoRouterRefreshStream(auth.stream),
  redirect: (context, state) => authRedirect(auth.state, state.matchedLocation),
  routes: [
    ShellRoute(
      builder: (_, state, child) =>
          AppShell(location: state.uri.path, child: child),
      routes: [
        GoRoute(
          path: '/',
          builder: (_, _) => BlocProvider(
            create: (_) => DashboardCubit(getIt())..load(),
            child: const DashboardPage(),
          ),
        ),
        GoRoute(
          path: '/orders',
          builder: (_, state) {
            final q = state.uri.queryParameters['tab'];
            final tab = orderTabs.contains(q) ? q! : 'all';
            return BlocProvider(
              key: ValueKey(tab), // dashboard theke onno tab e notun bloc
              create: (_) =>
                  OrderListBloc(getIt(), tab: tab)..add(OrderListStarted()),
              child: const OrderListPage(),
            );
          },
        ),
        GoRoute(
          path: '/orders/:id',
          builder: (_, state) => BlocProvider(
            create: (_) => OrderDetailCubit(
              getIt(),
              getIt<Browser>().openTab,
              _id(state, 'id'),
            )..load(),
            child: const OrderDetailPage(),
          ),
        ),
        GoRoute(
          path: '/packing',
          builder: (_, _) => BlocProvider(
            create: (_) =>
                PickListCubit(getIt(), getIt<Browser>().openTab)..load(),
            child: const PackingPage(),
          ),
        ),
        GoRoute(
          path: '/packing/:orderId/:fid',
          builder: (_, state) => BlocProvider(
            create: (_) =>
                PackCubit(getIt(), _id(state, 'orderId'), _id(state, 'fid'))
                  ..load(),
            child: const PackPage(),
          ),
        ),
        GoRoute(
          path: '/shipments',
          builder: (_, _) => BlocProvider(
            create: (_) => ShipmentsCubit(getIt())..load(),
            child: const ShipmentsPage(),
          ),
        ),
        GoRoute(
          path: '/products',
          builder: (_, _) => BlocProvider(
            create: (_) => ProductListCubit(getIt())..load(),
            child: const ProductListPage(),
          ),
        ),
        GoRoute(
          path: '/products/new',
          builder: (_, _) => BlocProvider(
            create: (_) => ProductFormCubit(getIt())..load(),
            child: const ProductFormPage(),
          ),
        ),
        GoRoute(
          path: '/products/:id',
          builder: (_, state) => BlocProvider(
            key: ValueKey(
              state.pathParameters['id'],
            ), // new → :id e notun cubit
            create: (_) =>
                ProductFormCubit(getIt(), id: _id(state, 'id'))..load(),
            child: const ProductFormPage(),
          ),
        ),
        GoRoute(
          path: '/categories',
          builder: (_, _) => BlocProvider(
            create: (_) => CategoriesCubit(getIt())..load(),
            child: const CategoriesPage(),
          ),
        ),
        GoRoute(
          path: '/inventory',
          builder: (_, state) {
            final low = state.uri.queryParameters['low'] == '1';
            return BlocProvider(
              key: ValueKey(low), // dashboard er low stock theke
              create: (_) => InventoryListCubit(getIt(), low: low)..load(),
              child: const InventoryPage(),
            );
          },
        ),
        GoRoute(
          path: '/inventory/:variantId',
          builder: (_, state) => BlocProvider(
            create: (_) =>
                VariantStockCubit(getIt(), _id(state, 'variantId'))..load(),
            child: VariantStockPage(
              variant: state.extra is InventoryVariant
                  ? state.extra as InventoryVariant
                  : null,
            ),
          ),
        ),
        GoRoute(
          path: '/staff',
          builder: (_, _) => BlocProvider(
            create: (_) => StaffCubit(getIt())..load(),
            child: const StaffPage(),
          ),
        ),
        GoRoute(
          path: '/coupons',
          builder: (_, _) => BlocProvider(
            create: (_) => CouponsCubit(getIt())..load(),
            child: const CouponsPage(),
          ),
        ),
        GoRoute(
          path: '/zones',
          builder: (_, _) => BlocProvider(
            create: (_) => ZonesCubit(getIt())..load(),
            child: const ZonesPage(),
          ),
        ),
        GoRoute(
          path: '/customers',
          builder: (_, _) => BlocProvider(
            create: (_) => CustomersCubit(getIt())..load(),
            child: const CustomersPage(),
          ),
        ),
        GoRoute(
          path: '/customers/:id',
          builder: (_, state) => BlocProvider(
            create: (_) =>
                CustomerDetailCubit(getIt(), _id(state, 'id'))..load(),
            child: const CustomerDetailPage(),
          ),
        ),
        GoRoute(
          path: '/audit',
          builder: (_, _) => BlocProvider(
            create: (_) => AuditCubit(getIt())..load(),
            child: const AuditPage(),
          ),
        ),
      ],
    ),
    GoRoute(path: '/login', builder: (_, _) => const LoginPage()),
    GoRoute(path: '/login/totp', builder: (_, _) => const TotpPage()),
    GoRoute(path: '/login/setup', builder: (_, _) => const SetupPage()),
  ],
  errorBuilder: (context, _) => Scaffold(
    body: Center(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(context.t('admin.not_found')),
          TextButton(
            onPressed: () => context.go('/'),
            child: Text(context.t('admin.home')),
          ),
        ],
      ),
    ),
  ),
);
