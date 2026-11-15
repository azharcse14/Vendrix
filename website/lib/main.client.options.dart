// dart format off
// ignore_for_file: type=lint

// GENERATED FILE, DO NOT MODIFY
// Generated with jaspr_builder

import 'package:jaspr/client.dart';

import 'package:website/components/add_to_cart.dart' deferred as _add_to_cart;
import 'package:website/components/filter_sheet.dart' deferred as _filter_sheet;
import 'package:website/components/login_form.dart' deferred as _login_form;
import 'package:website/components/variant_picker.dart'
    deferred as _variant_picker;
import 'package:website/components/wishlist_button.dart'
    deferred as _wishlist_button;

/// Default [ClientOptions] for use with your Jaspr project.
///
/// Use this to initialize Jaspr **before** calling [runApp].
///
/// Example:
/// ```dart
/// import 'main.client.options.dart';
///
/// void main() {
///   Jaspr.initializeApp(
///     options: defaultClientOptions,
///   );
///
///   runApp(...);
/// }
/// ```
ClientOptions get defaultClientOptions => ClientOptions(
  clients: {
    'add_to_cart': ClientLoader(
      (p) => _add_to_cart.AddToCart(digits: p['digits'] as String),
      loader: _add_to_cart.loadLibrary,
    ),
    'filter_sheet': ClientLoader(
      (p) => _filter_sheet.DialogToggle(p['dialogId'] as String),
      loader: _filter_sheet.loadLibrary,
    ),
    'login_form': ClientLoader(
      (p) => _login_form.LoginForm(next: p['next'] as String),
      loader: _login_form.loadLibrary,
    ),
    'variant_picker': ClientLoader(
      (p) => _variant_picker.VariantSwitch(),
      loader: _variant_picker.loadLibrary,
    ),
    'wishlist_button': ClientLoader(
      (p) => _wishlist_button.WishlistButton(loggedIn: p['loggedIn'] as bool),
      loader: _wishlist_button.loadLibrary,
    ),
  },
);
