// dart format off
// ignore_for_file: type=lint

// GENERATED FILE, DO NOT MODIFY
// Generated with jaspr_builder

import 'package:jaspr/server.dart';
import 'package:website/components/filter_sheet.dart' as _filter_sheet;
import 'package:website/components/variant_picker.dart' as _variant_picker;

/// Default [ServerOptions] for use with your Jaspr project.
///
/// Use this to initialize Jaspr **before** calling [runApp].
///
/// Example:
/// ```dart
/// import 'main.server.options.dart';
///
/// void main() {
///   Jaspr.initializeApp(
///     options: defaultServerOptions,
///   );
///
///   runApp(...);
/// }
/// ```
ServerOptions get defaultServerOptions => ServerOptions(
  clientId: 'main.client.dart.js',
  clients: {
    _filter_sheet.DialogToggle: ClientTarget<_filter_sheet.DialogToggle>(
      'filter_sheet',
      params: __filter_sheetDialogToggle,
    ),
    _variant_picker.VariantSwitch: ClientTarget<_variant_picker.VariantSwitch>(
      'variant_picker',
    ),
  },
);

Map<String, Object?> __filter_sheetDialogToggle(_filter_sheet.DialogToggle c) =>
    {'dialogId': c.dialogId};
