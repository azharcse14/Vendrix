import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:shared/shared.dart';

import '../../../core/auth/auth_bloc.dart';
import '../../../core/browser.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/error_retry.dart';
import '../../../core/widgets/note_dialog.dart';
import '../bloc/order_detail_cubit.dart';
import '../order_actions.dart';
import '../widgets/status_chip.dart';

/// `/orders/:id` (11.4): item (snapshot), taka er bhag, allowed button, history, note, print.
class OrderDetailPage extends StatelessWidget {
  const OrderDetailPage({super.key});

  @override
  Widget build(BuildContext context) {
    final s = context.watch<OrderDetailCubit>().state;
    return Scaffold(
      body: switch (s) {
        OrderDetailLoading() => const Center(
          child: CircularProgressIndicator(),
        ),
        OrderDetailFailed(:final error) => ErrorRetry(
          error,
          onRetry: context.read<OrderDetailCubit>().load,
        ),
        OrderDetailLoaded() => _Detail(s),
      },
    );
  }
}

class _Detail extends StatelessWidget {
  final OrderDetailLoaded s;
  const _Detail(this.s);

  @override
  Widget build(BuildContext context) {
    final o = s.order;
    final c = context.read<OrderDetailCubit>();
    final auth = context.watch<AuthBloc>().state;
    final role = auth is Authenticated ? auth.staff.role : '';
    final shop = !role.startsWith('vendor');
    final actions = orderActions(o, role);
    final text = Theme.of(context).textTheme;
    final phone = o.customerPhone ?? o.recipientPhone;
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        Wrap(
          spacing: 8,
          crossAxisAlignment: WrapCrossAlignment.center,
          children: [
            Text(o.orderNumber, style: text.titleLarge),
            StatusChip(o.displayStatus),
          ],
        ),
        Text(context.date(o.placedAt)),
        if (o.paymentMethod != null)
          Text(
            '${context.t('order.payment.${o.paymentMethod}')} · '
            '${context.t('print.payment_status.${o.paymentStatus}')}',
          ),
        if (o.deliveryDeadline != null)
          Text(
            context.t('order.deadline', {
              'date': context.date(o.deliveryDeadline!),
            }),
          ),
        if (s.error != null)
          Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(
              context.errorText(s.error!),
              style: TextStyle(color: Theme.of(context).colorScheme.error),
            ),
          ),
        const SizedBox(height: 12),
        Wrap(
          spacing: 8,
          runSpacing: 8,
          children: [
            if (actions.contains(OrderAction.confirm))
              FilledButton(
                key: const Key('order.confirm'),
                onPressed: s.busy ? null : () => c.transition('confirmed'),
                child: Text(context.t('admin.order.confirm')),
              ),
            if (actions.contains(OrderAction.cancel))
              OutlinedButton(
                key: const Key('order.cancel'),
                onPressed: s.busy
                    ? null
                    : () async {
                        final note = await showNoteDialog(
                          context,
                          title: 'admin.order.cancel',
                          required: false,
                        );
                        if (note != null) {
                          await c.transition(
                            'cancelled',
                            note: note.isEmpty ? null : note,
                          );
                        }
                      },
                child: Text(context.t('admin.order.cancel')),
              ),
            OutlinedButton(
              key: const Key('order.print.packing_slip'),
              onPressed: () => c.print('packing_slip'),
              child: Text(context.t('print.packing_slip')),
            ),
            // vendor role e invoice 403 (11.7 item 8)
            if (shop)
              OutlinedButton(
                key: const Key('order.print.invoice'),
                onPressed: () => c.print('invoice'),
                child: Text(context.t('print.invoice')),
              ),
          ],
        ),
        const Divider(height: 32),
        Text(context.t('order.recipient'), style: text.titleSmall),
        Row(
          children: [
            Expanded(
              child: Text(
                '${o.recipientName}\n$phone\n${o.address}, ${o.area}, ${o.district}',
              ),
            ),
            IconButton(
              tooltip: context.t('admin.orders.call'),
              onPressed: () => context.read<Browser>().open('tel:$phone'),
              icon: const Icon(Icons.call),
            ),
          ],
        ),
        if (o.note != null) Text('${context.t('print.note')}: ${o.note}'),
        for (final f in o.fulfillments)
          _FulfillmentCard(o, f, role, busy: s.busy),
        if (o.subtotal != null) ...[
          const SizedBox(height: 8),
          for (final (k, v) in [
            ('admin.order.subtotal', o.subtotal!),
            ('admin.order.discount', -o.discount!),
            ('admin.order.delivery', o.deliveryCharge!),
          ])
            _Amount(context.t(k), context.taka(v)),
        ],
        _Amount(
          context.t('admin.order.total'),
          context.taka(o.total),
          bold: true,
        ),
        const Divider(height: 32),
        Text(context.t('admin.order.history'), style: text.titleSmall),
        for (final h in o.history)
          ListTile(
            dense: true,
            contentPadding: EdgeInsets.zero,
            title: Text(context.t('order.status.${h.toStatus}')),
            subtitle: Text([context.date(h.at), ?h.note].join(' · ')),
          ),
        if (shop) ...[
          const Divider(height: 32),
          Text(context.t('admin.order.notes'), style: text.titleSmall),
          for (final n in o.notes)
            ListTile(
              dense: true,
              contentPadding: EdgeInsets.zero,
              leading: Icon(n.kind == 'call' ? Icons.call : Icons.notes),
              title: Text(n.note),
              subtitle: Text(context.date(n.createdAt)),
            ),
          Wrap(
            spacing: 8,
            children: [
              for (final kind in ['note', 'call'])
                TextButton.icon(
                  key: Key('order.add_$kind'),
                  onPressed: s.busy
                      ? null
                      : () async {
                          final note = await showNoteDialog(
                            context,
                            title: 'admin.order.add_$kind',
                          );
                          if (note != null) await c.addNote(note, kind: kind);
                        },
                  icon: Icon(kind == 'call' ? Icons.call : Icons.add),
                  label: Text(context.t('admin.order.add_$kind')),
                ),
            ],
          ),
        ],
      ],
    );
  }
}

class _Amount extends StatelessWidget {
  final String label, value;
  final bool bold;
  const _Amount(this.label, this.value, {this.bold = false});

  @override
  Widget build(BuildContext context) {
    final style = bold ? const TextStyle(fontWeight: FontWeight.bold) : null;
    return Row(
      children: [
        Expanded(child: Text(label, style: style)),
        Text(value, style: style),
      ],
    );
  }
}

class _FulfillmentCard extends StatelessWidget {
  final OrderDetail o;
  final Fulfillment f;
  final String role;
  final bool busy;
  const _FulfillmentCard(this.o, this.f, this.role, {required this.busy});

  @override
  Widget build(BuildContext context) {
    final c = context.read<OrderDetailCubit>();
    final actions = fulfillmentActions(o, f, role);
    Key key(FulfillmentAction a) => Key('fulfillment.${f.id}.${a.name}');
    Future<void> withNote(
      String title,
      String to, {
      required bool required,
    }) async {
      final note = await showNoteDialog(
        context,
        title: title,
        required: required,
      );
      if (note != null) {
        await c.transitionFulfillment(
          f.id,
          to,
          note: note.isEmpty ? null : note,
        );
      }
    }

    return Card(
      margin: const EdgeInsets.only(top: 12),
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Wrap(
              spacing: 8,
              crossAxisAlignment: WrapCrossAlignment.center,
              children: [
                Text(
                  context.t('admin.fulfillment.title', {
                    'seq': context.count(f.seq),
                  }),
                  style: Theme.of(context).textTheme.titleSmall,
                ),
                StatusChip(f.status),
              ],
            ),
            for (final i in f.items)
              ListTile(
                dense: true,
                contentPadding: EdgeInsets.zero,
                title: Text(
                  '${context.text(i.productName)} (${context.text(i.variantLabel)})',
                ),
                subtitle: Text(
                  [
                    '${context.count(i.quantity)} × ${context.taka(i.unitPrice)}',
                    if (i.packedWeightG ?? i.orderedWeightG case final g?)
                      context.t('print.grams', {'n': context.count(g)}),
                    if (i.status != 'active')
                      context.t('admin.item.${i.status}'),
                  ].join(' · '),
                ),
                trailing: Text(context.taka(i.lineTotal)),
              ),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                if (actions.contains(FulfillmentAction.pack))
                  FilledButton(
                    key: key(FulfillmentAction.pack),
                    onPressed: () => context.go('/packing/${o.id}/${f.id}'),
                    child: Text(context.t('admin.fulfillment.pack')),
                  ),
                if (actions.contains(FulfillmentAction.ship))
                  FilledButton(
                    key: key(FulfillmentAction.ship),
                    onPressed: () => context.go('/shipments'),
                    child: Text(context.t('admin.fulfillment.ship')),
                  ),
                if (actions.contains(FulfillmentAction.undo))
                  OutlinedButton(
                    key: key(FulfillmentAction.undo),
                    onPressed: busy
                        ? null
                        : () => withNote(
                            'admin.fulfillment.undo',
                            'open',
                            required: true,
                          ),
                    child: Text(context.t('admin.fulfillment.undo')),
                  ),
                if (actions.contains(FulfillmentAction.cancel))
                  OutlinedButton(
                    key: key(FulfillmentAction.cancel),
                    onPressed: busy
                        ? null
                        : () => withNote(
                            'admin.fulfillment.cancel',
                            'cancelled',
                            required: false,
                          ),
                    child: Text(context.t('admin.fulfillment.cancel')),
                  ),
              ],
            ),
          ],
        ),
      ),
    );
  }
}
