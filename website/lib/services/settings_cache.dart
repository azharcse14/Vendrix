import 'package:shared/shared.dart';

/// Ek shop er published settings ar translation override (3.5).
class ShopSettings {
  /// `GET /api/settings`.
  final PublicSettings settings;

  /// `GET /api/settings/version`.
  final int version;

  /// Locale (`bn`, `en`) → override.
  final Map<String, Map<String, String>> translations;

  /// ShopSettings.
  const ShopSettings(this.settings, this.version, this.translations);
}

/// Host onujayi memory cache (3.5, 10.10 item 4): [ttl] por prothom request purono dey ar background e version
/// check kore (host e ekbar e ekta); version bodlale settings ar duita translation abar. API down ba onno bhul
/// (`429` o) hole purono rakhe ar log; shop nai ba bondho hole bad. Faka cache e bhul [get] theke ber hoy (error page); ochena host
/// cache hoy na, tai memory API er chena host er beshi bare na.
class SettingsCache {
  /// Host er repository (request er IP chhara; background e o chole).
  final SettingsRepository Function(String host) repoFor;

  /// Version check er fank.
  final Duration ttl;

  final DateTime Function() _now;
  final void Function(Object error) _log;
  final _entries = <String, ({ShopSettings data, DateTime checkedAt})>{};
  final _loading = <String, Future<ShopSettings>>{};
  final _refreshing = <String>{};

  /// SettingsCache.
  SettingsCache(
    this.repoFor, {
    this.ttl = const Duration(seconds: 30),
    DateTime Function() now = DateTime.now,
    required void Function(Object error) log,
  }) : _now = now,
       _log = log;

  /// [host] er settings.
  Future<ShopSettings> get(String host) async {
    // API o eivabe chene (`resolve.go`); na hole ek shop er onek entry ar API call.
    host = host.toLowerCase().split(':').first;
    if (host.endsWith('.')) host = host.substring(0, host.length - 1);
    final e = _entries[host];
    if (e == null) {
      return _loading[host] ??= _load(host).whenComplete(() {
        _loading.remove(host);
      });
    }
    if (_now().difference(e.checkedAt) >= ttl && _refreshing.add(host)) {
      _refresh(host, e.data).whenComplete(() => _refreshing.remove(host));
    }
    return e.data;
  }

  Future<ShopSettings> _load(String host) async {
    final repo = repoFor(host);
    final version = await repo.version();
    final data = ShopSettings(await repo.settings(), version, {
      'bn': await repo.translations('bn'),
      'en': await repo.translations('en'),
    });
    _entries[host] = (data: data, checkedAt: _now());
    return data;
  }

  Future<void> _refresh(String host, ShopSettings old) async {
    try {
      if (await repoFor(host).version() != old.version) {
        await _load(host);
        return;
      }
      _entries[host] = (data: old, checkedAt: _now());
    } catch (err) {
      if (err is ApiError && (err.code == 'shop_not_found' || err.code == 'shop_suspended')) {
        _entries.remove(host);
      } else {
        _log(err);
        _entries[host] = (data: old, checkedAt: _now());
      }
    }
  }
}
