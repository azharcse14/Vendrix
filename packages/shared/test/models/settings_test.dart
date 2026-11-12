import 'package:shared/shared.dart';
import 'package:test/test.dart';

void main() {
  group('PublicSettings (9.2)', () {
    test('theme, branding er field', () {
      final s = PublicSettings.fromJson({
        'branding': {
          'name': {'bn': 'সবুজ বাজার'},
          'logo_key': 'shops/1/logo.png',
          'phone': '+8801712345678',
          'email': 'hello@example.com',
          'dbid_number': 'DBID-1',
          'bfsa_number': null,
        },
        'theme': {
          'fonts': {'heading': 'Baloo Da 2', 'body': 'Hind Siliguri'},
          'radius': 14,
        },
        'homepage': {
          'announcement': {'bn': 'ফ্রি ডেলিভারি'},
        },
      });
      expect(s.headingFont, 'Baloo Da 2');
      expect(s.bodyFont, 'Hind Siliguri');
      expect(s.radius, 14);
      expect(s.logoKey, 'shops/1/logo.png');
      expect(s.phone, '+8801712345678');
      expect(s.email, 'hello@example.com');
      expect(s.dbidNumber, 'DBID-1');
      expect(s.bfsaNumber, isNull);
      expect(s.announcement.of('en'), 'ফ্রি ডেলিভারি');
    });

    test('nai ba bhul type e default (Hind Siliguri, radius 8, null)', () {
      final s = PublicSettings.fromJson({
        'theme': {'fonts': 'x', 'radius': '14'},
        'branding': {'phone': 5},
      });
      expect(s.headingFont, 'Hind Siliguri');
      expect(s.bodyFont, 'Hind Siliguri');
      expect(s.radius, 8);
      expect(s.phone, isNull);
      expect(s.logoKey, isNull);
      expect(s.announcement.of('bn'), '');
      expect(s.noindex, isFalse);
      expect(s.ogImageKey, isNull);
    });

    test('seo: title, description, og_image_key, noindex', () {
      final s = PublicSettings.fromJson({
        'seo': {
          'title': {'bn': 'শিরোনাম', 'en': 'Title'},
          'description': {'bn': 'বিবরণ'},
          'og_image_key': 'shops/1/og.jpg',
          'noindex': true,
        },
      });
      expect(s.seoTitle.of('en'), 'Title');
      expect(s.seoDescription.of('en'), 'বিবরণ');
      expect(s.ogImageKey, 'shops/1/og.jpg');
      expect(s.noindex, isTrue);
    });

    test('homepage: banner, featured slug; bhul item bad', () {
      final s = PublicSettings.fromJson({
        'homepage': {
          'banners': [
            {
              'image_key': 'shops/1/b1.jpg',
              'alt': {'bn': 'অফার'},
              'link': '/c/chal',
            },
            {'image_key': 5},
            'x',
          ],
          'featured_category_slugs': ['chal', 7, 'modhu'],
          'featured_product_slugs': ['lal-chal'],
        },
      });
      expect(s.banners, hasLength(1));
      expect(s.banners.single.imageKey, 'shops/1/b1.jpg');
      expect(s.banners.single.alt.of('en'), 'অফার');
      expect(s.banners.single.link, '/c/chal');
      expect(s.featuredCategorySlugs, ['chal', 'modhu']);
      expect(s.featuredProductSlugs, ['lal-chal']);
      expect(const PublicSettings({}).banners, isEmpty);
      expect(const PublicSettings({}).featuredProductSlugs, isEmpty);
    });
  });
}
