import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:shared/shared.dart';

import '../../../core/browser.dart';
import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/order_list_bloc.dart';
import '../widgets/status_chip.dart';

/// `/orders` (11.4, 11.5): tab, number/phone khoja, risk, list (360px e ek column), ager/porer page.
class OrderListPage extends StatefulWidget {
  const OrderListPage({super.key});

  @override
  State<OrderListPage> createState() => _OrderListPageState();
}

class _OrderListPageState extends State<OrderListPage> {
  final _q = TextEditingController();

  @override
  void dispose() {
    _q.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final bloc = context.read<OrderListBloc>();
    final s = context.watch<OrderListBloc>().state;
    return Scaffold(
      body: Column(
        children: [
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 12, 16, 0),
            child: Row(
              children: [
                Expanded(
                  child: TextField(
                    key: const Key('orders.search'),
                    controller: _q,
                    decoration: InputDecoration(
                      labelText: context.t('admin.orders.search'),
                      prefixIcon: const Icon(Icons.search),
                      isDense: true,
                    ),
                    textInputAction: TextInputAction.search,
                    onSubmitted: (v) => bloc.add(OrderSearchSubmitted(v)),
                  ),
                ),
                const SizedBox(width: 8),
                FilterChip(
                  key: const Key('orders.risk'),
                  label: Text(context.t('admin.orders.risk')),
                  selected: s.filter.risk,
                  onSelected: (_) => bloc.add(OrderRiskToggled()),
                ),
              ],
            ),
          ),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 8),
            child: Row(
              children: [
                for (final t in orderTabs)
                  Padding(
                    padding: const EdgeInsets.symmetric(horizontal: 4),
                    child: ChoiceChip(
                      key: Key('orders.tab.$t'),
                      label: Text(
                        context.t(
                          orderTabs.indexOf(t) < 3
                              ? 'admin.orders.tab.$t'
                              : 'order.status.$t',
                        ),
                      ),
                      selected: s.filter.tab == t,
                      onSelected: (_) => bloc.add(OrderTabSelected(t)),
                    ),
                  ),
              ],
            ),
          ),
          Expanded(
            child: switch (s) {
              OrderListLoading() => const Center(
                child: CircularProgressIndicator(),
              ),
              OrderListFailed(:final error) => ErrorRetry(
                error,
                onRetry: () => bloc.add(OrderListStarted()),
              ),
              OrderListLoaded(:final page) when page.items.isEmpty => Center(
                child: Text(context.t('admin.orders.empty')),
              ),
              OrderListLoaded(:final page) => ListView(
                padding: const EdgeInsets.symmetric(horizontal: 12),
                children: [
                  for (final o in page.items) _OrderCard(o),
                  Row(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      IconButton(
                        key: const Key('orders.prev'),
                        tooltip: context.t('admin.prev'),
                        onPressed: page.page > 1
                            ? () => bloc.add(OrderPageRequested(page.page - 1))
                            : null,
                        icon: const Icon(Icons.chevron_left),
                      ),
                      Text(context.count(page.page)),
                      IconButton(
                        key: const Key('orders.next'),
                        tooltip: context.t('admin.next'),
                        onPressed: page.hasNext
                            ? () => bloc.add(OrderPageRequested(page.page + 1))
                            : null,
                        icon: const Icon(Icons.chevron_right),
                      ),
                    ],
                  ),
                ],
              ),
            },
          ),
        ],
      ),
    );
  }
}

class _OrderCard extends StatelessWidget {
  final AdminOrder o;
  const _OrderCard(this.o);

  @override
  Widget build(BuildContext context) {
    final phone = o.customerPhone ?? o.recipientPhone;
    final text = Theme.of(context).textTheme;
    return Card(
      clipBehavior: Clip.antiAlias,
      child: InkWell(
        onTap: () => context.go('/orders/${o.id}'),
        child: Padding(
          padding: const EdgeInsets.fromLTRB(12, 8, 4, 8),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Wrap(
                      spacing: 8,
                      crossAxisAlignment: WrapCrossAlignment.center,
                      children: [
                        Text(o.orderNumber, style: text.titleMedium),
                        StatusChip(o.displayStatus),
                      ],
                    ),
                    Text('${o.recipientName} · ${o.area}, ${o.district}'),
                    Text(
                      '${context.taka(o.total)} · ${context.date(o.placedAt)}',
                      style: text.bodySmall,
                    ),
                    if (o.riskFlags?.isNotEmpty ?? false)
                      Wrap(
                        spacing: 4,
                        runSpacing: 4,
                        children: [
                          for (final f in o.riskFlags!)
                            Chip(
                              key: Key('order.${o.id}.risk.$f'),
                              avatar: Icon(
                                Icons.warning_amber,
                                color: Theme.of(context).colorScheme.error,
                                size: 18,
                              ),
                              label: Text(context.t('admin.risk.$f')),
                              visualDensity: VisualDensity.compact,
                            ),
                        ],
                      ),
                  ],
                ),
              ),
              IconButton(
                key: Key('order.${o.id}.call'),
                tooltip: context.t('admin.orders.call'),
                onPressed: () => context.read<Browser>().open('tel:$phone'),
                icon: const Icon(Icons.call),
              ),
            ],
          ),
        ),
      ),
    );
  }
}
