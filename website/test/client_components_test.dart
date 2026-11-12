import 'dart:io';

import 'package:test/test.dart';

void main() {
  group('@client shudhu 10.2 er list', () {
    test('lib/ e @client thaka file shudhu filter sheet ar variant picker', () {
      final files = [
        for (final f in Directory('lib').listSync(recursive: true).whereType<File>())
          if (f.path.endsWith('.dart') && RegExp(r'^@client\b', multiLine: true).hasMatch(f.readAsStringSync()))
            f.path.replaceAll(r'\', '/'),
      ]..sort();
      expect(files, ['lib/components/filter_sheet.dart', 'lib/components/variant_picker.dart']);
    });
  });
}
