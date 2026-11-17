/// Bechhe neya file (bytes, naam).
typedef PickedFile = ({List<int> bytes, String name});

/// Browser er kaj (web e `package:web` diye `di.dart` e; test e fake).
class Browser {
  /// Notun tab: tap er shomoy-i khole (Safari async `window.open` atkay), [url] ele sekhane jay; bhul hole bondho.
  final void Function(Future<String> url) openTab;

  /// Ei tab theke shoja (jemon `tel:`).
  final void Function(String url) open;

  /// Chobi file (JPEG/PNG) bechhe neya; bondho korle null. Type/size server abar dekhe (11.7 item 9).
  final Future<PickedFile?> Function() pickFile;

  /// Browser.
  const Browser({
    required this.openTab,
    required this.open,
    required this.pickFile,
  });
}
