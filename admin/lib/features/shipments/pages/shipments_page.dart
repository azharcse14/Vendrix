import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/shipments_cubit.dart';

/// `/shipments` (11.5: 360px e list, boro button): pathano baki ar pothe.
class ShipmentsPage extends StatelessWidget {
  const ShipmentsPage({super.key});

  @override
  Widget build(BuildContext context) {
    final c = context.read<ShipmentsCubit>();
    final s = context.watch<ShipmentsCubit>().state;
    final auth = context.watch<AuthBloc>().state;
    // vendor role e shipped → delivered na (CanTransitionFulfillment: Staff, System)
    final deliver =
        auth is Authenticated && !auth.staff.role.startsWith('vendor');
    final text = Theme.of(context).textTheme;
    return Scaffold(
      body: switch (s) {
        ShipmentsLoading() => const Center(child: CircularProgressIndicator()),
        ShipmentsFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        ShipmentsLoaded() => RefreshIndicator(
          onRefresh: c.load,
          child: ListView(
            padding: const EdgeInsets.all(16),
            children: [
              if (s.error != null)
                Text(
                  _error(context, s.error!),
                  style: TextStyle(color: Theme.of(context).colorScheme.error),
                ),
              Text(
                context.t('admin.shipments.to_ship'),
                style: text.titleSmall,
              ),
              if (s.toShip.isEmpty) Text(context.t('admin.shipments.empty')),
              for (final r in s.toShip)
                _Row(
                  r,
                  actions: [
                    FilledButton(
                      key: Key('ship.${r.id}'),
                      onPressed: s.busy ? null : () => _shipDialog(context, r),
                      child: Text(context.t('admin.fulfillment.ship')),
                    ),
                  ],
                ),
              const SizedBox(height: 16),
              Text(
                context.t('admin.shipments.on_the_way'),
                style: text.titleSmall,
              ),
              if (s.onTheWay.isEmpty) Text(context.t('admin.shipments.empty')),
              for (final r in s.onTheWay)
                _Row(
                  r,
                  detail: [
                    context.t('admin.carrier.${r.shipment!.carrier}'),
                    ?r.shipment!.trackingCode,
                    if (r.shipment!.codAmount! > 0)
                      context.t('admin.shipments.cod', {
                        'amount': context.taka(r.shipment!.codAmount!),
                      }),
                  ],
                  actions: [
                    if (deliver)
                      FilledButton(
                        key: Key('shipment.${r.shipment!.id}.delivered'),
                        onPressed: s.busy
                            ? null
                            : () =>
                                  c.updateShipment(r.shipment!.id, 'delivered'),
                        child: Text(context.t('admin.shipments.delivered')),
                      ),
                    for (final st in ['returned', 'cancelled'])
                      OutlinedButton(
                        key: Key('shipment.${r.shipment!.id}.$st'),
                        onPressed: s.busy
                            ? null
                            : () => c.updateShipment(r.shipment!.id, st),
                        child: Text(context.t('admin.shipments.$st')),
                      ),
                  ],
                ),
            ],
          ),
        ),
      },
    );
  }

  String _error(BuildContext context, ApiError e) =>
      e.fields['tracking_code'] == 'taken'
      ? context.t('admin.shipments.tracking_taken')
      : context.errorText(e);

  Future<void> _shipDialog(BuildContext context, FulfillmentRow r) async {
    final c = context.read<ShipmentsCubit>();
    final res = await showDialog<(String, String)>(
      context: context,
      builder: (_) => _ShipDialog(r.orderNumber),
    );
    if (res != null) await c.ship(r.id, res.$1, tracking: res.$2);
  }
}

class _Row extends StatelessWidget {
  final FulfillmentRow r;
  final List<String> detail;
  final List<Widget> actions;
  const _Row(this.r, {this.detail = const [], required this.actions});

  @override
  Widget build(BuildContext context) => Card(
    margin: const EdgeInsets.only(top: 8),
    child: Padding(
      padding: const EdgeInsets.all(12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(r.orderNumber, style: Theme.of(context).textTheme.titleMedium),
          Text('${r.recipientName} · ${r.area}, ${r.district}'),
          for (final d in detail) Text(d),
          const SizedBox(height: 8),
          Wrap(spacing: 8, runSpacing: 8, children: actions),
        ],
      ),
    ),
  );
}

class _ShipDialog extends StatefulWidget {
  final String orderNumber;
  const _ShipDialog(this.orderNumber);

  @override
  State<_ShipDialog> createState() => _ShipDialogState();
}

class _ShipDialogState extends State<_ShipDialog> {
  var _carrier = carriers.first;
  final _tracking = TextEditingController();

  @override
  void dispose() {
    _tracking.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) => AlertDialog(
    title: Text(context.t('order.title', {'number': widget.orderNumber})),
    content: Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        DropdownButtonFormField<String>(
          key: const Key('ship.carrier'),
          isExpanded: true,
          initialValue: _carrier,
          decoration: InputDecoration(
            labelText: context.t('admin.shipments.carrier'),
          ),
          items: [
            for (final c in carriers)
              DropdownMenuItem(
                value: c,
                child: Text(context.t('admin.carrier.$c')),
              ),
          ],
          onChanged: (v) => setState(() => _carrier = v!),
        ),
        const SizedBox(height: 12),
        TextField(
          key: const Key('ship.tracking'),
          controller: _tracking,
          maxLength: 40,
          decoration: InputDecoration(
            labelText: context.t('admin.shipments.tracking'),
          ),
        ),
      ],
    ),
    actions: [
      TextButton(
        onPressed: () => Navigator.of(context).pop(),
        child: Text(context.t('admin.close')),
      ),
      FilledButton(
        key: const Key('ship.submit'),
        onPressed: () => Navigator.of(context).pop((_carrier, _tracking.text)),
        child: Text(context.t('admin.fulfillment.ship')),
      ),
    ],
  );
}
