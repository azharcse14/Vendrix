// Icon er path Lucide (lucide-static v0.460.0, https://lucide.dev) theke hubohu (10.5 v1.38 item 1).
//
// ISC License
//
// Copyright (c) for portions of Lucide are held by Cole Bemis 2013-2022 as part of Feather (MIT). All other
// copyright (c) for Lucide are held by Lucide Contributors 2022.
//
// Permission to use, copy, modify, and/or distribute this software for any purpose with or without fee is hereby
// granted, provided that the above copyright notice and this permission notice appear in all copies.
//
// THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE INCLUDING
// ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL,
// DIRECT, INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS,
// WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE
// USE OR PERFORMANCE OF THIS SOFTWARE.

import 'package:jaspr/dom.dart';
import 'package:jaspr/jaspr.dart';

/// Icon er naam (plan 3.4); value Lucide er SVG er bhetorer markup.
enum Icons {
  /// Lucide `house`.
  home(
    '<path d="M15 21v-8a1 1 0 0 0-1-1h-4a1 1 0 0 0-1 1v8"/><path d="M3 10a2 2 0 0 1 .709-1.528l7-5.999a2 2 0 0 1 '
    '2.582 0l7 5.999A2 2 0 0 1 21 10v9a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z"/>',
  ),

  /// Lucide `layout-grid`.
  category(
    '<rect width="7" height="7" x="3" y="3" rx="1"/><rect width="7" height="7" x="14" y="3" rx="1"/>'
    '<rect width="7" height="7" x="14" y="14" rx="1"/><rect width="7" height="7" x="3" y="14" rx="1"/>',
  ),

  /// Lucide `shopping-cart`.
  cart(
    '<circle cx="8" cy="21" r="1"/><circle cx="19" cy="21" r="1"/>'
    '<path d="M2.05 2.05h2l2.66 12.42a2 2 0 0 0 2 1.58h9.78a2 2 0 0 0 1.95-1.57l1.65-7.43H5.12"/>',
  ),

  /// Lucide `user`.
  account('<path d="M19 21v-2a4 4 0 0 0-4-4H9a4 4 0 0 0-4 4v2"/><circle cx="12" cy="7" r="4"/>'),

  /// Lucide `search`.
  search('<circle cx="11" cy="11" r="8"/><path d="m21 21-4.3-4.3"/>'),

  /// Lucide `leaf`.
  leaf(
    '<path d="M11 20A7 7 0 0 1 9.8 6.1C15.5 5 17 4.48 19 2c1 2 2 4.18 2 8 0 5.5-4.78 10-10 10Z"/>'
    '<path d="M2 21c0-3 1.85-5.36 5.08-6C9.5 14.52 12 13 13 12"/>',
  ),

  /// Lucide `truck`.
  truck(
    '<path d="M14 18V6a2 2 0 0 0-2-2H4a2 2 0 0 0-2 2v11a1 1 0 0 0 1 1h2"/><path d="M15 18H9"/>'
    '<path d="M19 18h2a1 1 0 0 0 1-1v-3.65a1 1 0 0 0-.22-.624l-3.48-4.35A1 1 0 0 0 17.52 8H14"/>'
    '<circle cx="17" cy="18" r="2"/><circle cx="7" cy="18" r="2"/>',
  ),

  /// Lucide `banknote` (COD).
  cash(
    '<rect width="20" height="12" x="2" y="6" rx="2"/><circle cx="12" cy="12" r="2"/><path d="M6 12h.01M18 12h.01"/>',
  ),

  /// Lucide `shield-check` (certification).
  certified(
    '<path d="M20 13c0 5-3.5 7.5-7.66 8.95a1 1 0 0 1-.67-.01C7.5 20.5 4 18 4 13V6a1 1 0 0 1 1-1c2 0 4.5-1.2 '
    '6.24-2.72a1.17 1.17 0 0 1 1.52 0C14.51 3.81 17 5 19 5a1 1 0 0 1 1 1z"/><path d="m9 12 2 2 4-4"/>',
  ),

  /// Lucide `check`.
  check('<path d="M20 6 9 17l-5-5"/>'),

  /// Lucide `phone`.
  phone(
    '<path d="M22 16.92v3a2 2 0 0 1-2.18 2 19.79 19.79 0 0 1-8.63-3.07 19.5 19.5 0 0 1-6-6 19.79 19.79 0 0 1-3.07-8.67'
    'A2 2 0 0 1 4.11 2h3a2 2 0 0 1 2 1.72 12.84 12.84 0 0 0 .7 2.81 2 2 0 0 1-.45 2.11L8.09 9.91a16 16 0 0 0 6 6l1.27-'
    '1.27a2 2 0 0 1 2.11-.45 12.84 12.84 0 0 0 2.81.7A2 2 0 0 1 22 16.92z"/>',
  ),

  /// Lucide `chevron-down`.
  chevronDown('<path d="m6 9 6 6 6-6"/>'),

  /// Lucide `chevron-right`.
  chevronRight('<path d="m9 18 6-6-6-6"/>'),

  /// Lucide `heart`.
  heart(
    '<path d="M19 14c1.49-1.46 3-3.21 3-5.5A5.5 5.5 0 0 0 16.5 3c-1.76 0-3 .5-4.5 2-1.5-1.5-2.74-2-4.5-2'
    'A5.5 5.5 0 0 0 2 8.5c0 2.3 1.5 4.05 3 5.5l7 7Z"/>',
  );

  const Icons(this.markup);

  /// SVG er bhetorer markup (constant, user er lekha na).
  final String markup;
}

/// Inline SVG icon (font icon ba JS na): rong lekhar `currentColor`, screen reader e lukano; mane lekha ba
/// button er `aria-label` e.
class Icon extends StatelessComponent {
  /// Kon icon.
  final Icons icon;

  /// Size ar onno class.
  final String classes;

  /// Icon.
  const Icon(this.icon, {this.classes = 'size-5 shrink-0', super.key});

  @override
  Component build(BuildContext context) => svg(
    viewBox: '0 0 24 24',
    classes: classes,
    attributes: const {
      'fill': 'none',
      'stroke': 'currentColor',
      'stroke-width': '2',
      'stroke-linecap': 'round',
      'stroke-linejoin': 'round',
      'aria-hidden': 'true',
    },
    [RawText(icon.markup)],
  );
}
