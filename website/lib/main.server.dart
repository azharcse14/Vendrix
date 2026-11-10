/// Website er server entry (Jaspr server mode, 10.2).
library;

import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';

import 'app.dart';
import 'main.server.options.dart';

void main() {
  Jaspr.initializeApp(options: defaultServerOptions);
  runApp(
    Document(
      lang: 'bn',
      viewport: 'width=device-width, initial-scale=1, viewport-fit=cover',
      head: [link(rel: 'stylesheet', href: '/styles.css')],
      body: const App(),
    ),
  );
}
