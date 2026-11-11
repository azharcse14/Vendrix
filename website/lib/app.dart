import 'package:jaspr/server.dart';
import 'package:jaspr_router/jaspr_router.dart';

import 'components/layout.dart';
import 'pages/home.dart';
import 'pages/status.dart';

/// Route (10.1): protiti route Bangla (root) ar English (`/en`) duitai; baki page M3 er porer step e.
class App extends StatelessComponent {
  const App({super.key});

  @override
  Component build(BuildContext context) {
    List<Route> both(String path, Component page) => [
      Route(path: path, builder: (context, state) => page),
      Route(path: path == '/' ? '/en' : '/en$path', builder: (context, state) => page),
    ];
    return Layout(
      child: Router(
        routes: [...both('/', const Home())],
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
