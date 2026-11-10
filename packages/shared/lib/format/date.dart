import 'package:intl/date_symbol_data_local.dart';
import 'package:intl/intl.dart';

import 'money.dart';

// ponytail: shudhu Bangladesh (DST nai); onno timezone er shop (MT) ele package:timezone ba design-change.
const _offsets = {'Asia/Dhaka': Duration(hours: 6)};

var _ready = false;

/// Tarikh shop er timezone e (Rule 5, 9.8, v1.12): "২৭ অক্টোবর ২০২৬". Mash er naam UI bhasha theke,
/// digit [numberLocale] er niyome. Ochena `timeZone` e [ArgumentError].
String formatDate(
  DateTime t, {
  required String uiLocale,
  required bool banglaDigits,
  String timeZone = 'Asia/Dhaka',
}) {
  final offset = _offsets[timeZone];
  if (offset == null) throw ArgumentError.value(timeZone, 'timeZone');
  if (!_ready) {
    // local data, tai sathe sathe shesh hoy (Future er opekkha lage na)
    initializeDateFormatting();
    _ready = true;
  }
  final f = DateFormat('d MMMM y', uiLocale == 'bn' ? 'bn' : 'en')
    ..useNativeDigits =
        numberLocale(uiLocale, banglaDigits: banglaDigits) == 'bn';
  return f.format(t.toUtc().add(offset));
}
