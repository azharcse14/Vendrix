import 'package:flutter/material.dart';

import '../../../core/l10n/locale_cubit.dart';

/// `display_status` ba fulfillment status er lekha (`order.status.*`, shared).
class StatusChip extends StatelessWidget {
  final String status;
  const StatusChip(this.status, {super.key});

  @override
  Widget build(BuildContext context) => Chip(
    label: Text(context.t('order.status.$status')),
    visualDensity: VisualDensity.compact,
  );
}
