import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

/// Admin/vendor er lekha (description, policy) plain text e (10.10 item 2, 12.2): shob escape (`text`), khali line →
/// `<p>`, newline → `<br>`; link ba HTML na.
List<Component> plainText(String s, {String classes = 'mb-3'}) => [
  for (final para in s.split(RegExp(r'\n[ \t]*\n')))
    if (para.trim().isNotEmpty)
      p(classes: classes, [
        for (final (i, line) in para.trim().split('\n').indexed) ...[if (i > 0) br(), .text(line)],
      ]),
];
