// dart format off
// ignore_for_file: type=lint

// GENERATED FILE, DO NOT MODIFY
// Generated with jaspr_builder

import 'package:jaspr/server.dart';
import 'package:website/components/add_to_cart.dart' as _add_to_cart;
import 'package:website/components/filter_sheet.dart' as _filter_sheet;
import 'package:website/components/login_form.dart' as _login_form;
import 'package:website/components/variant_picker.dart' as _variant_picker;
import 'package:website/components/wishlist_button.dart' as _wishlist_button;

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
    _add_to_cart.AddToCart: ClientTarget<_add_to_cart.AddToCart>(
      'add_to_cart',
      params: __add_to_cartAddToCart,
    ),
    _filter_sheet.DialogToggle: ClientTarget<_filter_sheet.DialogToggle>(
      'filter_sheet',
      params: __filter_sheetDialogToggle,
    ),
    _login_form.LoginForm: ClientTarget<_login_form.LoginForm>(
      'login_form',
      params: __login_formLoginForm,
    ),
    _variant_picker.VariantSwitch: ClientTarget<_variant_picker.VariantSwitch>(
      'variant_picker',
    ),
    _wishlist_button.WishlistButton:
        ClientTarget<_wishlist_button.WishlistButton>(
          'wishlist_button',
          params: __wishlist_buttonWishlistButton,
        ),
  },
);

Map<String, Object?> __add_to_cartAddToCart(_add_to_cart.AddToCart c) => {
  'digits': c.digits,
};
Map<String, Object?> __filter_sheetDialogToggle(_filter_sheet.DialogToggle c) =>
    {'dialogId': c.dialogId};
Map<String, Object?> __login_formLoginForm(_login_form.LoginForm c) => {
  'next': c.next,
};
Map<String, Object?> __wishlist_buttonWishlistButton(
  _wishlist_button.WishlistButton c,
) => {'loggedIn': c.loggedIn};
