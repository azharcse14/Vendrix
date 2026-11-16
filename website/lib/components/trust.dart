import 'icon.dart';
import 'site.dart';

/// Trust (10.5 v1.38 item 3, home ar product): shudhu settings e ja shotto. Delivery din na (zone er notun API call
/// lagto); certification ekhane na (settings e shop er certificate nai, shudhu product e).
List<(Icons, String)> shopTrust(Site site) {
  final s = site.settings;
  final days = s.returnWindowDays;
  return [
    if (s.paymentMethods.contains('cod')) (Icons.cash, site.t('trust.cod')),
    if (days != null) (Icons.check, site.translator.plural('trust.return', days, {'count': site.number(days)})),
  ];
}
