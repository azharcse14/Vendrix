import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

// `#RRGGBB` (deriveTokens) → opaque Color.
Color _color(String hex) =>
    Color(int.parse('FF${hex.substring(1)}', radix: 16));

// Latin NotoSans, Bangla okkhor fallback e NotoSansBengali (duitai asset; CanvasKit CDN theke font na ane).
const _fallback = ['NotoSansBengali'];

/// 9.7: token theke `ThemeData`. Font asset (11.7 item 10), shop er font M8 e.
ThemeData buildTheme(Map<String, String> tokens, {double radius = 14}) {
  final t = tokens.map((k, v) => MapEntry(k, _color(v)));
  final shape = RoundedRectangleBorder(
    borderRadius: BorderRadius.circular(radius),
  );
  return ThemeData(
    colorScheme: ColorScheme.light(
      primary: t['primary']!,
      onPrimary: t['on-primary']!,
      secondary: t['accent']!,
      onSecondary: t['on-accent']!,
      surface: t['background']!,
      onSurface: t['text']!,
      onSurfaceVariant: t['text-muted']!,
      outline: t['border']!,
      error: t['danger']!,
    ),
    scaffoldBackgroundColor: t['background'],
    fontFamily: 'NotoSans',
    fontFamilyFallback: _fallback,
    cardTheme: CardThemeData(color: t['surface'], shape: shape),
    filledButtonTheme: FilledButtonThemeData(
      style: FilledButton.styleFrom(
        shape: shape,
        minimumSize: const Size(64, 48), // 11.5: phone e boro button
      ),
    ),
    inputDecorationTheme: InputDecorationTheme(
      border: OutlineInputBorder(borderRadius: BorderRadius.circular(radius)),
    ),
  );
}

/// App er theme (11.3); ekhon organic preset (9.4), shop er theme M8 e.
class ThemeCubit extends Cubit<ThemeData> {
  ThemeCubit() : super(buildTheme(deriveTokens(organicColors)));
}
