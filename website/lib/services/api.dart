import 'package:http/http.dart' as http;
import 'package:shared/shared.dart';

import 'config.dart';

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

/// Protiti page request e notun [ApiClient] (10.2); bhasha URL theke ([locale]), browser er header na.
ApiClient apiFor(Map<String, String> request, String locale, Config cfg, http.Client client) =>
    ApiClient(cfg.apiBaseUrl, client, locale: locale, headers: apiHeaders(request, cfg));
