import 'package:flutter/material.dart';

import '../l10n/locale_cubit.dart';

/// Note chay emon kaj (cancel, undo, ojon beshi): note dile trim kora lekha, bondho korle null.
/// [title], [hint] translation key (dialog er build e lekha; callback e `context.t` na). [required] hole khali
/// note e submit na.
Future<String?> showNoteDialog(
  BuildContext context, {
  required String title,
  String? hint,
  bool required = true,
}) => showDialog<String>(
  context: context,
  builder: (_) => _NoteDialog(title: title, hint: hint, required: required),
);

class _NoteDialog extends StatefulWidget {
  final String title;
  final String? hint;
  final bool required;
  const _NoteDialog({required this.title, this.hint, required this.required});

  @override
  State<_NoteDialog> createState() => _NoteDialogState();
}

class _NoteDialogState extends State<_NoteDialog> {
  final _form = GlobalKey<FormState>();
  final _note = TextEditingController();

  @override
  void dispose() {
    _note.dispose();
    super.dispose();
  }

  void _submit() {
    if (_form.currentState!.validate()) {
      Navigator.of(context).pop(_note.text.trim());
    }
  }

  @override
  Widget build(BuildContext context) {
    final req = context.t('admin.required'); // validator callback e watch na
    return AlertDialog(
      title: Text(context.t(widget.title)),
      content: Form(
        key: _form,
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            if (widget.hint != null) Text(context.t(widget.hint!)),
            TextFormField(
              key: const Key('note.field'),
              controller: _note,
              autofocus: true,
              maxLength: 500,
              decoration: InputDecoration(labelText: context.t('admin.note')),
              validator: (v) =>
                  widget.required && (v ?? '').trim().isEmpty ? req : null,
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
          key: const Key('note.submit'),
          onPressed: _submit,
          child: Text(context.t('admin.save')),
        ),
      ],
    );
  }
}
