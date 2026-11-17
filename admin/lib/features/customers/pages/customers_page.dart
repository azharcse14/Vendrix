import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/customers_cubit.dart';

/// `/customers`: customer khoja ba block kora phone (notun block, unblock).
class CustomersPage extends StatelessWidget {
  const CustomersPage({super.key});

  Future<void> _block(BuildContext context) async {
    final c = context.read<CustomersCubit>();
    final r = await showDialog<(String, String)>(
      context: context,
      builder: (_) => const _BlockDialog(),
    );
    if (r != null) await c.block(r.$1, r.$2.isEmpty ? null : r.$2);
  }

  Future<void> _unblock(BuildContext context, String phone) async {
    final c = context.read<CustomersCubit>();
    if (await confirmDialog(context, 'admin.customer.unblock_confirm')) {
      await c.unblock(phone);
    }
  }

  @override
  Widget build(BuildContext context) {
    final c = context.read<CustomersCubit>();
    final s = context.watch<CustomersCubit>().state;
    final router = GoRouter.maybeOf(context);
    final busy = s is CustomersLoaded && s.busy;
    return Scaffold(
      body: ListView(
        padding: const EdgeInsets.all(16),
        children: [
          Wrap(
            spacing: 8,
            children: [
              for (final blocked in [false, true])
                ChoiceChip(
                  key: Key(blocked ? 'customers.blocked' : 'customers.all'),
                  label: Text(
                    context.t(
                      blocked ? 'admin.customer.blocked' : 'admin.customer.all',
                    ),
                  ),
                  selected: c.showBlocked == blocked,
                  onSelected: (_) => c.tab(blocked: blocked),
                ),
            ],
          ),
          const SizedBox(height: 8),
          if (c.showBlocked)
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: FilledButton.icon(
                key: const Key('blocked.new'),
                onPressed: busy ? null : () => _block(context),
                icon: const Icon(Icons.block),
                label: Text(context.t('admin.customer.block')),
              ),
            )
          else
            TextField(
              key: const Key('customers.search'),
              textInputAction: TextInputAction.search,
              maxLength: 64,
              decoration: InputDecoration(
                labelText: context.t('admin.customer.search'),
                prefixIcon: const Icon(Icons.search),
                counterText: '',
              ),
              onSubmitted: c.search,
            ),
          ...switch (s) {
            CustomersLoading() => [
              const Center(child: CircularProgressIndicator()),
            ],
            CustomersFailed(:final error) => [
              ErrorRetry(error, onRetry: c.load),
            ],
            final CustomersLoaded s => [
              if (s.error != null) ErrorLine(context.errorText(s.error!)),
              if ((s.customers?.items ?? s.blocked!.items).isEmpty)
                Text(context.t('admin.empty')),
              for (final cu in s.customers?.items ?? <Customer>[])
                Card(
                  margin: const EdgeInsets.only(top: 8),
                  child: ListTile(
                    key: Key('customer.${cu.id}'),
                    title: Text(cu.name ?? cu.phone),
                    subtitle: Text(
                      [
                        cu.phone,
                        context.t('admin.customer.orders', {
                          'n': context.count(cu.orderCount ?? 0),
                          'amount': context.taka(cu.orderTotal ?? 0),
                        }),
                        if (cu.blocked) context.t('admin.customer.is_blocked'),
                      ].join(' · '),
                    ),
                    onTap: () => router?.go('/customers/${cu.id}'),
                  ),
                ),
              for (final b in s.blocked?.items ?? <BlockedPhone>[])
                Card(
                  margin: const EdgeInsets.only(top: 8),
                  child: ListTile(
                    title: Text(b.phone),
                    subtitle: Text(
                      [?b.reason, context.date(b.createdAt)].join(' · '),
                    ),
                    trailing: IconButton(
                      key: Key('blocked.${b.phone}.unblock'),
                      tooltip: context.t('admin.customer.unblock'),
                      onPressed: s.busy
                          ? null
                          : () => _unblock(context, b.phone),
                      icon: const Icon(Icons.lock_open),
                    ),
                  ),
                ),
              PageBar(
                prefix: 'customers',
                page: s.customers?.page ?? s.blocked!.page,
                hasNext: s.customers?.hasNext ?? s.blocked!.hasNext,
                onPage: c.load,
              ),
            ],
          },
        ],
      ),
    );
  }
}

class _BlockDialog extends StatefulWidget {
  const _BlockDialog();

  @override
  State<_BlockDialog> createState() => _BlockDialogState();
}

class _BlockDialogState extends State<_BlockDialog> {
  final _form = GlobalKey<FormState>();
  final _phone = TextEditingController();
  final _reason = TextEditingController();

  @override
  void dispose() {
    _phone.dispose();
    _reason.dispose();
    super.dispose();
  }

  void _submit() {
    if (_form.currentState!.validate()) {
      Navigator.of(context)
          .pop((normalizePhone(_phone.text)!, _reason.text.trim()));
    }
  }

  @override
  Widget build(BuildContext context) {
    final badPhone = context.t('error.invalid_phone');
    return AlertDialog(
      title: Text(context.t('admin.customer.block')),
      content: Form(
        key: _form,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            TextFormField(
              key: const Key('block.phone'),
              controller: _phone,
              keyboardType: TextInputType.phone,
              decoration: InputDecoration(labelText: context.t('login.phone')),
              validator: (v) =>
                  normalizePhone(v ?? '') == null ? badPhone : null,
            ),
            TextFormField(
              key: const Key('block.reason'),
              controller: _reason,
              maxLength: 200,
              decoration: InputDecoration(
                labelText: context.t('admin.customer.reason'),
              ),
            ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.of(context).pop(),
          child: Text(context.t('admin.close')),
        ),
        FilledButton(
          key: const Key('block.save'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
