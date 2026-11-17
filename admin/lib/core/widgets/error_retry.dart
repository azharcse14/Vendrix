import 'package:flutter/material.dart';
import 'package:shared/shared.dart';

import '../l10n/locale_cubit.dart';

/// Load er bhul: lekha ar "abar cheshta".
class ErrorRetry extends StatelessWidget {
  final ApiError error;
  final VoidCallback onRetry;
  const ErrorRetry(this.error, {super.key, required this.onRetry});

  @override
  Widget build(BuildContext context) => Center(
    child: Padding(
      padding: const EdgeInsets.all(16),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          Text(context.errorText(error), textAlign: TextAlign.center),
          const SizedBox(height: 12),
          OutlinedButton(
            onPressed: onRetry,
            child: Text(context.t('admin.retry')),
          ),
        ],
      ),
    ),
  );
}
