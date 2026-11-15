import 'package:jaspr/server.dart';
import 'package:jaspr_router/jaspr_router.dart';

import 'components/layout.dart';
import 'pages/account.dart';
import 'pages/cart.dart';
import 'pages/category.dart';
import 'pages/home.dart';
import 'pages/login.dart';
import 'pages/policy.dart';
import 'pages/product.dart';
import 'pages/search.dart';
import 'pages/status.dart';
import 'pages/wishlist.dart';

/// Route (10.1): protiti route Bangla (root) ar English (`/en`) duitai.
class App extends StatelessComponent {
  const App({super.key});

  @override
  Component build(BuildContext context) {
    List<Route> both(String path, Component Function(RouteState state) page) => [
      Route(path: path, builder: (context, state) => page(state)),
      Route(path: path == '/' ? '/en' : '/en$path', builder: (context, state) => page(state)),
    ];
    return Layout(
      child: Router(
        routes: [
          ...both('/', (_) => const Home()),
          ...both('/c/:slug', (s) => CategoryPage(s.params['slug']!, s.queryParams)),
          ...both('/p/:slug', (s) => ProductPage(s.params['slug']!, query: s.queryParams)),
          ...both('/cart', (s) => CartPage(s.queryParams)),
          ...both('/search', (s) => SearchPage(s.queryParams)),
          ...both('/login', (s) => LoginPage(s.queryParams)),
          ...both('/account', (s) => AccountPage(s.queryParams)),
          ...both('/account/wishlist', (_) => const WishlistPage()),
          // Ochena kind e API call na, sheshe router er 404.
          for (final kind in policyKinds) ...both('/policy/$kind', (_) => PolicyPage(kind)),
        ],
        errorBuilder: (context, state) => const _NotFound(),
      ),
    );
  }
}

// Router e route na mille 404 status shoho.
class _NotFound extends StatelessComponent {
  const _NotFound();

  @override
  Component build(BuildContext context) {
    context.setStatusCode(404);
    return const StatusPage(StatusKind.notFound);
  }
}
