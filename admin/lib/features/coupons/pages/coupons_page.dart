import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/common.dart';
import '../../../core/widgets/error_retry.dart';
import '../bloc/coupons_cubit.dart';
import '../widgets/coupon_dialog.dart';

/// `/coupons`: list (notun age), notun, edit, active switch.
class CouponsPage extends StatelessWidget {
  const CouponsPage({super.key});

  Future<void> _edit(BuildContext context, Coupon? cp) async {
    final c = context.read<CouponsCubit>();
    final body = await showCouponDialog(context, cp);
    if (body != null) await c.save(cp?.id, body);
  }

  // `fields` er karon (lock, below_used) age, na hole code er lekha (7.1)
  String _error(BuildContext context, ApiError e) =>
      e.fields['usage_limit'] == 'below_used'
      ? context.t('admin.coupon.below_used')
      : e.fields.containsValue('locked')
      ? context.t('admin.coupon.locked')
      : context.errorText(e);

  @override
  Widget build(BuildContext context) {
    final c = context.read<CouponsCubit>();
    return Scaffold(
      body: switch (context.watch<CouponsCubit>().state) {
        CouponsLoading() => const Center(child: CircularProgressIndicator()),
        CouponsFailed(:final error) => ErrorRetry(error, onRetry: c.load),
        final CouponsLoaded s => ListView(
          padding: const EdgeInsets.all(16),
          children: [
            if (s.error != null) ErrorLine(_error(context, s.error!)),
            Align(
              alignment: AlignmentDirectional.centerStart,
              child: FilledButton.icon(
                key: const Key('coupon.new'),
                onPressed: s.busy ? null : () => _edit(context, null),
                icon: const Icon(Icons.add),
                label: Text(context.t('admin.coupon.new')),
              ),
            ),
            if (s.page.items.isEmpty) Text(context.t('admin.empty')),
            for (final cp in s.page.items)
              Card(
                key: Key('coupon.${cp.id}'),
                margin: const EdgeInsets.only(top: 8),
                child: ListTile(
                  title: Text(cp.code),
                  subtitle: Text(
                    [
                      cp.kind == 'percent'
                          ? context.t('admin.coupon.percent_off', {
                              'n': context.count(cp.value),
                            })
                          : context.taka(cp.value),
                      if (cp.minOrder > 0)
                        context.t('admin.zone.min_order_is', {
                          'amount': context.taka(cp.minOrder),
                        }),
                      context.t('admin.coupon.used_count', {
                        'n': context.count(cp.usedCount),
                        'limit': cp.usageLimit == null
                            ? '∞'
                            : context.count(cp.usageLimit!),
                      }),
                      if (cp.expiresAt != null)
                        context.t('admin.coupon.until', {
                          'date': context.date(cp.expiresAt!),
                        }),
                    ].join(' · '),
                  ),
                  trailing: Row(
                    mainAxisSize: MainAxisSize.min,
                    children: [
                      Switch(
                        key: Key('coupon.${cp.id}.active'),
                        value: cp.isActive,
                        onChanged: s.busy ? null : (v) => c.setActive(cp.id, v),
                      ),
                      IconButton(
                        key: Key('coupon.${cp.id}.edit'),
                        tooltip: context.t('admin.edit'),
                        onPressed: s.busy ? null : () => _edit(context, cp),
                        icon: const Icon(Icons.edit_outlined),
                      ),
                    ],
                  ),
                ),
              ),
            PageBar(
              prefix: 'coupons',
              page: s.page.page,
              hasNext: s.page.hasNext,
              onPage: c.load,
            ),
          ],
        ),
      },
    );
  }
}
