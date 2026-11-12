import 'package:test/test.dart';
import 'package:website/services/image.dart';

void main() {
  group('Chobi URL (10.5, 10.10 item 3)', () {
    final cf = ImageUrls(Uri.parse('https://img.example.com/'), resize: true);
    final plain = ImageUrls(Uri.parse('http://127.0.0.1:8081/dev-images'), resize: false);

    test('cloudflare: width ar format=auto, srcset 320–1280', () {
      expect(
        cf.url('shops/1/rice.jpg', 640),
        'https://img.example.com/cdn-cgi/image/width=640,format=auto/shops/1/rice.jpg',
      );
      expect(
        cf.srcset('shops/1/rice.jpg'),
        [
          320,
          640,
          960,
          1280,
        ].map((w) => 'https://img.example.com/cdn-cgi/image/width=$w,format=auto/shops/1/rice.jpg ${w}w').join(', '),
      );
    });

    test('none: shudhu base/key, srcset nai', () {
      expect(plain.url('shops/1/rice.jpg', 640), 'http://127.0.0.1:8081/dev-images/shops/1/rice.jpg');
      expect(plain.srcset('shops/1/rice.jpg'), isNull);
    });

    test('key er segment encode (space, ?, #); ".." ba khali segment e null', () {
      expect(cf.url('shops/1/a b?#.jpg', 320), endsWith('/shops/1/a%20b%3F%23.jpg'));
      expect(cf.url('shops/../x.jpg', 320), isNull);
      expect(cf.url('/shops/1/x.jpg', 320), isNull);
      expect(cf.url('', 320), isNull);
    });
  });
}
