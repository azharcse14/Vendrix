import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:go_router/go_router.dart';

import '../../../core/l10n/locale_cubit.dart';
import '../../../core/widgets/error_retry.dart';
import '../../../core/widgets/note_dialog.dart';
import '../bloc/pack_cubit.dart';

/// Ek fulfillment pack (11.4, 11.5): fixed item dekhay, ojon-variable e gram; `weight_over_limit` e note dialog.
class PackPage extends StatefulWidget {
  const PackPage({super.key});

  @override
  State<PackPage> createState() => _PackPageState();
}

class _PackPageState extends State<PackPage> {
  final _form = GlobalKey<FormState>();
  final _grams = <int, TextEditingController>{};

  @override
  void dispose() {
    for (final c in _grams.values) {
      c.dispose();
    }
    super.dispose();
  }

  Map<int, int> get _weights => {
    for (final e in _grams.entries) e.key: int.parse(e.value.text),
  };

  void _submit() {
    if (_form.currentState!.validate()) {
      context.read<PackCubit>().submit(_weights);
    }
  }

  Future<void> _overLimit(BuildContext context) async {
    final note = await showNoteDialog(
      context,
      title: 'admin.pack.over_title',
      hint: 'admin.pack.over_hint',
    );
    if (note != null && context.mounted) {
      await context.read<PackCubit>().submit(_weights, customerOkNote: note);
    }
  }

  @override
  Widget build(BuildContext context) {
    final s = context.watch<PackCubit>().state;
    final invalid = context.t(
      'admin.pack.weight_invalid',
    ); // validator e watch na
    return BlocListener<PackCubit, PackState>(
      listenWhen: (_, s) => s is PackReady && s.overLimit && !s.busy,
      listener: (context, _) => _overLimit(context),
      child: Scaffold(
        body: switch (s) {
          PackLoading() => const Center(child: CircularProgressIndicator()),
          PackFailed(:final error) => ErrorRetry(
            error,
            onRetry: context.read<PackCubit>().load,
          ),
          PackDone() => Center(
            child: Column(
              mainAxisSize: MainAxisSize.min,
              children: [
                Text(context.t('admin.pack.done')),
                const SizedBox(height: 12),
                FilledButton(
                  onPressed: () => context.go('/packing'),
                  child: Text(context.t('admin.nav.packing')),
                ),
              ],
            ),
          ),
          PackReady() => Form(
            key: _form,
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                Text(
                  context.t('order.title', {'number': s.order.orderNumber}),
                  style: Theme.of(context).textTheme.titleLarge,
                ),
                Text(
                  '${s.order.recipientName} · ${s.order.area}, ${s.order.district}',
                ),
                const SizedBox(height: 12),
                for (final i in s.fulfillment.items.where(
                  (i) => i.status == 'active',
                ))
                  Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: i.variableWeight
                        ? TextFormField(
                            key: Key('pack.item.${i.id}'),
                            controller: _grams.putIfAbsent(
                              i.id,
                              TextEditingController.new,
                            ),
                            keyboardType: TextInputType.number,
                            inputFormatters: [
                              FilteringTextInputFormatter.digitsOnly,
                            ],
                            decoration: InputDecoration(
                              labelText:
                                  '${context.text(i.productName)} (${context.text(i.variantLabel)})',
                              helperText: context.t('admin.pack.ordered', {
                                'n': context.count(i.orderedWeightG ?? 0),
                              }),
                              suffixText: context.t('admin.pack.grams'),
                            ),
                            validator: (v) {
                              final g = int.tryParse(v ?? '');
                              return g == null || g < 1 || g > 1000000
                                  ? invalid
                                  : null;
                            },
                          )
                        : ListTile(
                            contentPadding: EdgeInsets.zero,
                            leading: const Icon(Icons.check_box_outline_blank),
                            title: Text(
                              '${context.text(i.productName)} (${context.text(i.variantLabel)})',
                            ),
                            trailing: Text('× ${context.count(i.quantity)}'),
                          ),
                  ),
                if (s.error != null && !s.overLimit)
                  Padding(
                    padding: const EdgeInsets.only(bottom: 12),
                    child: Text(
                      context.errorText(s.error!),
                      style: TextStyle(
                        color: Theme.of(context).colorScheme.error,
                      ),
                    ),
                  ),
                FilledButton(
                  key: const Key('pack.submit'),
                  onPressed: s.busy ? null : _submit,
                  child: Text(context.t('admin.pack.submit')),
                ),
              ],
            ),
          ),
        },
      ),
    );
  }
}
