import 'dart:io';

/// Website er env (13.5, 10.10). Shuru tei pora hoy; nai ba bhul hole server uthe na.
class Config {
  /// Go API (Cloudflare diye na, loopback).
  final Uri apiBaseUrl;

  /// API te `X-Internal-Auth` (shudhu `X-Client-IP` er sathe).
  final String internalSecret;

  /// Chobir base URL (platform er, Rule 15).
  final Uri imageBaseUrl;

  /// `IMAGE_RESIZE=cloudflare` hole `/cdn-cgi/image/...`; `none` e shudhu `<base>/<key>`.
  final bool imageResize;

  const Config._(this.apiBaseUrl, this.internalSecret, this.imageBaseUrl, this.imageResize);

  /// [env] theke; bhul hole [FormatException] e variable er naam.
  factory Config.fromEnv(Map<String, String> env) {
    Uri url(String name) {
      final u = Uri.tryParse(env[name] ?? '');
      if (u == null || !u.isScheme('http') && !u.isScheme('https') || u.host.isEmpty) {
        throw FormatException('$name: http(s) URL lagbe');
      }
      return u;
    }

    final api = url('API_BASE_URL');
    final secret = env['JASPR_INTERNAL_SECRET'] ?? '';
    if (secret.isEmpty) throw const FormatException('JASPR_INTERNAL_SECRET lagbe');
    final images = url('IMAGE_BASE_URL');
    final resize = switch (env['IMAGE_RESIZE']) {
      'cloudflare' => true,
      'none' => false,
      _ => throw const FormatException('IMAGE_RESIZE: cloudflare ba none'),
    };
    return Config._(api, secret, images, resize);
  }
}

/// Process er env theke (prothom use e).
final config = Config.fromEnv(Platform.environment);
