import 'dart:convert';

import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';
import 'package:test/test.dart';
import 'package:website/services/settings_cache.dart';

void main() {
  late DateTime now;
  late Map<String, int> version; // host → published version
  late List<String> calls; // "host path?locale"
  late bool down;
  late int? status; // API er bhul (jemon 423, 429)
  late SettingsCache cache;
  late List<Object> logged;

  SettingsRepository repoFor(String host) => SettingsRepository(
    ApiClient(
      Uri.parse('http://api'),
      MockClient((req) async {
        calls.add('$host ${req.url.path}${req.url.hasQuery ? '?${req.url.query}' : ''}');
        if (down) throw http.ClientException('down');
        if (status case final st?) {
          final code = st == 423 ? 'shop_suspended' : 'rate_limited';
          return http.Response('{"error":{"code":"$code","message":""}}', st);
        }
        final v = version[host];
        if (v == null) {
          return http.Response('{"error":{"code":"shop_not_found","message":""}}', 404);
        }
        final body = switch (req.url.path) {
          '/api/settings/version' => {'version': v},
          '/api/settings' => {
            'branding': {
              'name': {'bn': '$host v$v'},
            },
          },
          _ => {'k': '${req.url.queryParameters['locale']} v$v'},
        };
        return http.Response(jsonEncode(body), 200);
      }),
    ),
  );

  setUp(() {
    now = DateTime.utc(2026, 11, 10);
    version = {'a.shop': 1, 'b.shop': 7};
    calls = [];
    down = false;
    status = null;
    logged = [];
    cache = SettingsCache(repoFor, now: () => now, log: logged.add);
  });

  // background refresh shesh howar jonno
  Future<void> settle() => Future.delayed(Duration.zero);

  group('Settings cache (3.5)', () {
    test('prothom request e settings, version ar duita translation; porer request API te jay na', () async {
      final s = await cache.get('a.shop');
      expect(s.version, 1);
      expect(s.settings.name.of('bn'), 'a.shop v1');
      expect(s.translations['en'], {'k': 'en v1'});
      expect(s.translations['bn'], {'k': 'bn v1'});
      expect(calls.toSet(), {
        'a.shop /api/settings/version',
        'a.shop /api/settings',
        'a.shop /api/translations?locale=bn',
        'a.shop /api/translations?locale=en',
      });
      calls.clear();
      now = now.add(const Duration(seconds: 29));
      await cache.get('a.shop');
      expect(calls, isEmpty);
    });

    test('30 second por version check; na bodlale shudhu version call', () async {
      await cache.get('a.shop');
      calls.clear();
      now = now.add(const Duration(seconds: 31));
      await cache.get('a.shop');
      await settle();
      expect(calls, ['a.shop /api/settings/version']);
      calls.clear();
      await cache.get('a.shop'); // checkedAt notun, abar check na
      await settle();
      expect(calls, isEmpty);
    });

    test('version bodlale request atkay na (purono dey), background e notun; porer request notun', () async {
      await cache.get('a.shop');
      version['a.shop'] = 2;
      now = now.add(const Duration(seconds: 31));
      expect((await cache.get('a.shop')).version, 1);
      await settle();
      final s = await cache.get('a.shop');
      expect(s.version, 2);
      expect(s.settings.name.of('bn'), 'a.shop v2');
      expect(s.translations['bn'], {'k': 'bn v2'});
    });

    test('ekshathe onek request e refresh ekta-i', () async {
      await cache.get('a.shop');
      calls.clear();
      now = now.add(const Duration(seconds: 31));
      await Future.wait([for (var i = 0; i < 5; i++) cache.get('a.shop')]);
      await settle();
      expect(calls.where((c) => c.endsWith('/version')), hasLength(1));
    });

    test('prothom load o ekshathe ekta-i', () async {
      await Future.wait([for (var i = 0; i < 5; i++) cache.get('a.shop')]);
      expect(calls.where((c) => c.endsWith('/api/settings')), hasLength(1));
    });

    test('host onujayi alada', () async {
      expect((await cache.get('a.shop')).version, 1);
      expect((await cache.get('b.shop')).settings.name.of('bn'), 'b.shop v7');
      expect((await cache.get('a.shop')).settings.name.of('bn'), 'a.shop v1');
    });

    test('API down: purono cache dey ar log; 30 second por abar cheshta', () async {
      await cache.get('a.shop');
      down = true;
      now = now.add(const Duration(seconds: 31));
      expect((await cache.get('a.shop')).version, 1);
      await settle();
      expect(logged, hasLength(1));
      calls.clear();
      await cache.get('a.shop');
      await settle();
      expect(calls, isEmpty);
    });

    test('cache faka ar API down: ApiError (error page), cache e kichu na', () async {
      down = true;
      await expectLater(cache.get('a.shop'), throwsA(isA<ApiError>().having((e) => e.code, 'code', 'network_error')));
      down = false;
      expect((await cache.get('a.shop')).version, 1);
    });

    test('ochena host cache hoy na', () async {
      for (var i = 0; i < 2; i++) {
        await expectLater(
          cache.get('x.shop'),
          throwsA(isA<ApiError>().having((e) => e.code, 'code', 'shop_not_found')),
        );
      }
      expect(calls.where((c) => c == 'x.shop /api/settings/version'), hasLength(2));
    });

    test('refresh e shop nai (404) hole cache theke bad; porer request error', () async {
      await cache.get('a.shop');
      version.remove('a.shop');
      now = now.add(const Duration(seconds: 31));
      await cache.get('a.shop');
      await settle();
      await expectLater(cache.get('a.shop'), throwsA(isA<ApiError>()));
    });

    test('refresh e 429: purono rakhe ar log; 423 shop_suspended: bad', () async {
      await cache.get('a.shop');
      status = 429;
      now = now.add(const Duration(seconds: 31));
      await cache.get('a.shop');
      await settle();
      expect(logged, hasLength(1));
      expect((await cache.get('a.shop')).version, 1);
      status = 423;
      now = now.add(const Duration(seconds: 31));
      await cache.get('a.shop');
      await settle();
      await expectLater(cache.get('a.shop'), throwsA(isA<ApiError>().having((e) => e.code, 'code', 'shop_suspended')));
    });
  });
}
