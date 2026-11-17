/// Browser er kaj (web e `package:web` diye `di.dart` e; test e fake).
class Browser {
  /// Notun tab: tap er shomoy-i khole (Safari async `window.open` atkay), [url] ele sekhane jay; bhul hole bondho.
  final void Function(Future<String> url) openTab;

  /// Ei tab theke shoja (jemon `tel:`).
  final void Function(String url) open;

  /// Browser.
  const Browser({required this.openTab, required this.open});
}
