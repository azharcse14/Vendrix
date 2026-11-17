import 'package:flutter/material.dart';

import '../l10n/locale_cubit.dart';

/// Taka (poisha) → form er lekha (`120.5` na, `120.50`; purno hole `120`).
String takaInput(int? p) => p == null
    ? ''
    : p % 100 == 0
    ? '${p ~/ 100}'
    : '${p ~/ 100}.${'${p % 100}'.padLeft(2, '0')}';

final _date = RegExp(r'^\d{4}-\d{2}-\d{2}$');

/// `YYYY-MM-DD` ar asol din (2026-13-40 na).
bool validDate(String s) {
  if (!_date.hasMatch(s)) return false;
  final d = DateTime.tryParse(s);
  return d != null && d.toIso8601String().startsWith(s);
}

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
