/// Website er server entry (Jaspr server mode, 10.2).
library;

import 'dart:io';

import 'package:jaspr/dom.dart';
import 'package:jaspr/server.dart';
import 'package:shelf/shelf.dart';
import 'package:shelf/shelf_io.dart' as shelf_io;

import 'app.dart';
import 'main.server.options.dart';
import 'services/api.dart';
import 'services/bff.dart';
import 'services/cache_control.dart';
import 'services/config.dart';

Future<void> main() async {
  Jaspr.initializeApp(options: defaultServerOptions);
  final handler = const Pipeline()
      .addMiddleware(cacheControl())
      .addMiddleware(bff(config, httpClient, bffRoutes))
      .addHandler(
        serveApp(
          (request, render) => render(
            Document(
              lang: 'bn',
              viewport: 'width=device-width, initial-scale=1, viewport-fit=cover',
              head: [link(rel: 'stylesheet', href: '/styles.css')],
              body: const App(),
            ),
          ),
        ),
      );
  // Shudhu loopback (13.5): Caddy chhara keu `X-Real-IP` dite parbe na. `jaspr serve`/build `PORT` dey.
  final server = await shelf_io.serve(
    handler,
    InternetAddress.loopbackIPv4,
    int.parse(Platform.environment['PORT'] ?? '8081'),
  );
  print('Serving at http://localhost:${server.port}');
}
