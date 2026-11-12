/// Chobir width (10.5 niyom 7).
const imageWidths = [320, 640, 960, 1280];

/// `storage_key` → URL (10.10 item 3): `IMAGE_RESIZE=cloudflare` e `<base>/cdn-cgi/image/width=W,format=auto/<key>`,
/// `none` e `<base>/<key>`.
class ImageUrls {
  /// `IMAGE_BASE_URL` (platform er, Rule 15).
  final Uri base;

  /// Cloudflare resizing.
  final bool resize;

  /// ImageUrls.
  const ImageUrls(this.base, {required this.resize});

  /// [width] er URL; key e `..`, khali segment ba `/` shuru hole `null` (onno path e jete dey na).
  String? url(String key, int width) {
    final parts = key.split('/');
    if (parts.any((p) => p.isEmpty || p == '.' || p == '..')) return null;
    final b = base.toString().replaceFirst(RegExp(r'/+$'), '');
    final k = parts.map(Uri.encodeComponent).join('/');
    return resize ? '$b/cdn-cgi/image/width=$width,format=auto/$k' : '$b/$k';
  }

  /// `srcset` ([imageWidths]); resize na hole `null` (ekta-i file).
  String? srcset(String key) {
    if (!resize || url(key, imageWidths.first) == null) return null;
    return imageWidths.map((w) => '${url(key, w)} ${w}w').join(', ');
  }
}
