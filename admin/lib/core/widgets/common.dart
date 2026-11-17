import 'package:flutter/material.dart';

import '../l10n/locale_cubit.dart';

/// Haa/na dialog (delete, reset); [title] translation key. Haa hole true.
Future<bool> confirmDialog(BuildContext context, String title) async =>
    await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(context.t(title)),
        actions: [
          TextButton(
            onPressed: () => Navigator.of(context).pop(false),
            child: Text(context.t('admin.close')),
          ),
          FilledButton(
            key: const Key('confirm.ok'),
            onPressed: () => Navigator.of(context).pop(true),
            child: Text(context.t('admin.confirm')),
          ),
        ],
      ),
    ) ??
    false;

/// List er ager/porer page (OFFSET paging); [prefix] key er jonno (`products.next`).
class PageBar extends StatelessWidget {
  final String prefix;
  final int page;
  final bool hasNext;
  final ValueChanged<int> onPage;
  const PageBar({
    super.key,
    required this.prefix,
    required this.page,
    required this.hasNext,
    required this.onPage,
  });

  @override
  Widget build(BuildContext context) => Row(
    mainAxisAlignment: MainAxisAlignment.center,
    children: [
      IconButton(
        key: Key('$prefix.prev'),
        tooltip: context.t('admin.prev'),
        onPressed: page > 1 ? () => onPage(page - 1) : null,
        icon: const Icon(Icons.chevron_left),
      ),
      Text(context.count(page)),
      IconButton(
        key: Key('$prefix.next'),
        tooltip: context.t('admin.next'),
        onPressed: hasNext ? () => onPage(page + 1) : null,
        icon: const Icon(Icons.chevron_right),
      ),
    ],
  );
}

/// Page er upore kaj er bhul (data thake).
class ErrorLine extends StatelessWidget {
  final String text;
  const ErrorLine(this.text, {super.key});

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: 8),
    child: Text(
      text,
      style: TextStyle(color: Theme.of(context).colorScheme.error),
    ),
  );
}
