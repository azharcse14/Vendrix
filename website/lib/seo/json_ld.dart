import 'dart:convert';

import 'package:shared/shared.dart';

/// Poisha → `850.00` (OG `product:price:amount`, JSON-LD `price`). Shudhu lekha, hishab na (Rule 1).
String amount(int poisha) => '${poisha ~/ 100}.${(poisha % 100).toString().padLeft(2, '0')}';

/// `<script type="application/ld+json">` er bhetor (10.4, 12.2): `<` JSON er unicode escape e, tai naam e `</script>` thakleo
/// script bondho hoy na; JSON hishebe ekoi.
String jsonLd(Map<String, Object?> data) => jsonEncode(data).replaceAll('<', r'\u003c');

/// Product er JSON-LD (10.4 item 3): protiti variant ekta `Offer` (API er dam, stock); `aggregateRating` shudhu
/// review thakle (Google khali rating e bhul dhore).
Map<String, Object?> productJsonLd(Product p, {required String url, required List<String> images}) => {
  '@context': 'https://schema.org',
  '@type': 'Product',
  'name': p.name,
  if (p.description.isNotEmpty) 'description': p.description,
  if (images.isNotEmpty) 'image': images,
  'offers': [
    for (final v in p.variants)
      {
        '@type': 'Offer',
        'sku': v.sku,
        'price': amount(v.price),
        'priceCurrency': 'BDT',
        'availability': v.inStock ? 'https://schema.org/InStock' : 'https://schema.org/OutOfStock',
        'url': url,
      },
  ],
  if (p.reviewCount > 0)
    'aggregateRating': {'@type': 'AggregateRating', 'ratingValue': p.rating, 'reviewCount': p.reviewCount},
};
