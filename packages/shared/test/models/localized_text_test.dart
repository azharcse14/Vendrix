import 'package:shared/shared.dart';
import 'package:test/test.dart';

void main() {
  group('LocalizedText (9.8, 7.1)', () {
    test('value[locale], khali ba nai hole bn', () {
      final t = LocalizedText.fromJson({'bn': 'লাল চাল', 'en': 'Red Rice'});
      expect(t.of('en'), 'Red Rice');
      expect(t.of('bn'), 'লাল চাল');
      expect(t.of('fr'), 'লাল চাল');
      expect(LocalizedText.fromJson({'bn': 'মধু', 'en': ''}).of('en'), 'মধু');
      expect(LocalizedText.fromJson({}).of('en'), '');
      expect(t.toJson(), {'bn': 'লাল চাল', 'en': 'Red Rice'});
    });
  });
}
