---
paths:
  - "packages/shared/**"
---
# packages/shared: pure Dart (Rule 10, 4.3)
- Import shudhu Dart core (`dart:convert`, `dart:async`, `dart:math` ...), `package:http`, `package:intl`. `package:flutter`, `dart:io`, `dart:html`, `dart:ui` nishiddho.
- Taka `int` poisha. Format `NumberFormat('#,##,##0', locale)` + samne ৳ nije; `NumberFormat.currency` na (9.8). Test: `৳১,২৫,০০০`, `৳১৮০.৫০`, `৳1,25,000`.
- Translator er fallback kram 9.8 er moto; theme token derive ar contrast 9.3 er formula (4.5:1).
- API client e HTTP client inject hoy (admin e `BrowserClient`); error e logic `code` diye, `message` shudhu dekhano (7.1).
- Website, admin, app eta path dependency diye ney (`shared: {path: ../packages/shared}`, 4.6); pub.dev e publish na. `analysis_options.yaml` e `public_member_api_docs` lint (shob app er shared code, tai protiti public jinish e doc comment).
