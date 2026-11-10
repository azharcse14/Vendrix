import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

/// Home (10.1): ekhon shudhu skeleton; data ar layout step 6–8 e.
class Home extends StatelessComponent {
  const Home({super.key});

  @override
  Component build(BuildContext context) {
    return main_(classes: 'min-h-screen bg-page text-ink font-body', [
      h1(classes: 'font-heading text-primary', [.text('Vendrix')]),
    ]);
  }
}
