import 'package:jaspr/jaspr.dart';
import 'package:jaspr_router/jaspr_router.dart';

import 'pages/home.dart';

/// Route (10.1); `/en` prefix ar baki page M3 er porer step e.
class App extends StatelessComponent {
  const App({super.key});

  @override
  Component build(BuildContext context) {
    return Router(
      routes: [Route(path: '/', builder: (context, state) => const Home())],
    );
  }
}
