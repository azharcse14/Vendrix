import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

import '../components/site.dart';

/// Home (10.1): ekhon shudhu shop er naam; data step 8 e.
class Home extends StatelessComponent {
  const Home({super.key});

  @override
  Component build(BuildContext context) {
    final site = Site.of(context);
    return div(classes: 'px-4 py-8', [
      h1([.text(site.settings.name.of(site.locale))]),
    ]);
  }
}
