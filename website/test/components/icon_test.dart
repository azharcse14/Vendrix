import 'dart:convert';

import 'package:html/parser.dart' show parseFragment;
import 'package:jaspr/server.dart';
import 'package:test/test.dart';
import 'package:website/components/icon.dart';
import 'package:website/main.server.options.dart';

void main() {
  setUpAll(() => Jaspr.initializeApp(options: defaultServerOptions));

  group('Icon (10.5 v1.38 item 1: Lucide inline SVG)', () {
    test('protiti naam: inline svg, currentColor, aria-hidden, path ache', () async {
      expect(Icons.values, hasLength(greaterThanOrEqualTo(12)));
      for (final name in Icons.values) {
        final res = await renderComponent(Icon(name), standalone: true);
        final svg = parseFragment(utf8.decode(res.body)).querySelector('svg');
        expect(svg, isNotNull, reason: name.name);
        final a = svg!.attributes;
        expect(a['viewBox'], '0 0 24 24', reason: name.name);
        expect(a['stroke'], 'currentColor', reason: name.name);
        expect(a['fill'], 'none', reason: name.name);
        expect(a['aria-hidden'], 'true', reason: name.name);
        expect(svg.querySelectorAll('path, circle, rect'), isNotEmpty, reason: name.name);
      }
    });

    test('plan 3.4 er naam gulo ache', () {
      expect(
        Icons.values.map((i) => i.name),
        containsAll([
          'home',
          'category',
          'cart',
          'account',
          'search',
          'leaf',
          'truck',
          'cash',
          'certified',
          'check',
          'phone',
          'chevronDown',
        ]),
      );
    });

    test('class bodlano jay (size)', () async {
      final res = await renderComponent(const Icon(Icons.cart, classes: 'size-6'), standalone: true);
      expect(parseFragment(utf8.decode(res.body)).querySelector('svg')!.classes, contains('size-6'));
    });
  });
}
