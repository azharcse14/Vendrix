import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_web_plugins/url_strategy.dart';

import 'di.dart';

Future<void> main() async {
  usePathUrlStrategy();
  WidgetsFlutterBinding.ensureInitialized();
  // Semantics shob shomoy (screen reader, WebDriver; 11.7 item 10).
  SemanticsBinding.instance.ensureSemantics();
  runApp(await bootstrap());
}
