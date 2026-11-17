import 'package:flutter/widgets.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import 'bn.dart' as admin_bn;
import 'en.dart' as admin_en;

/// UI bhasha `bn` ba `en` (11.3); memory te, default `bn`.
class LocaleCubit extends Cubit<String> {
  LocaleCubit() : super('bn');

  void toggle() => emit(state == 'bn' ? 'en' : 'bn');
}

// shared er lekha (`error.*`) + admin er `admin.*` (11.7 item 10).
final _translators = {
  for (final l in ['bn', 'en'])
    l: Translator(
      l,
      defaults: {
        'bn': {...defaultMessages['bn']!, ...admin_bn.messages},
        'en': {...defaultMessages['en']!, ...admin_en.messages},
      },
      onMissing: (k, l) => debugPrint('missing translation: $l $k'),
    ),
};

/// `context.t('admin.login.title')`; shudhu `build` e (LocaleCubit `watch` kore).
extension L10n on BuildContext {
  String t(String key, [Map<String, Object> args = const {}]) =>
      _translators[watch<LocaleCubit>().state]!.t(key, args);

  /// API bhul er lekha: `error.<code>`.
  String errorText(ApiError e) => t('error.${e.code}');
}
