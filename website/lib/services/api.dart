import 'dart:io';

import 'package:http/http.dart' as http;
import 'package:shared/shared.dart';

import 'config.dart';
import 'settings_cache.dart';

/// Shob request e ek HTTP client (connection reuse).
final httpClient = http.Client();

/// Request theke API header (3.2): shop `Host` theke (website nije slug khoje na); Caddy er `X-Real-IP` thakle
/// `X-Client-IP` + `X-Internal-Auth`, na thakle (local) API connection IP ney. [request] er key lowercase.
Map<String, String> apiHeaders(Map<String, String> request, Config cfg) {
  final ip = request['x-real-ip'] ?? '';
  return {
    'X-Shop-Host': request['host'] ?? '',
    if (ip.isNotEmpty) ...{'X-Client-IP': ip, 'X-Internal-Auth': cfg.internalSecret},
  };
}

/// Request er origin (absolute URL er jonno): Caddy `X-Forwarded-Proto: https` dey, local e `http`. Host ochena hole
/// API age-i `shop_not_found` dey, tai page e ashe na.
String originOf(Map<String, String> request) =>
    '${request['x-forwarded-proto'] == 'https' ? 'https' : 'http'}://${(request['host'] ?? '').toLowerCase()}';

/// Protiti page request e notun [ApiClient] (10.2); bhasha URL theke ([locale]), browser er header na.
ApiClient apiFor(Map<String, String> request, String locale, Config cfg, http.Client client) =>
    ApiClient(cfg.apiBaseUrl, client, locale: locale, headers: apiHeaders(request, cfg));

/// Process er settings cache (3.5); background refresh e customer er IP nai, API connection IP ney.
final settingsCache = SettingsCache(
  (host) => SettingsRepository(ApiClient(config.apiBaseUrl, httpClient, headers: {'X-Shop-Host': host})),
  log: (e) => stderr.writeln('settings refresh: $e'),
);
