import 'package:admin/features/products/bloc/product_form_cubit.dart';
import 'package:admin/features/products/bloc/product_list_cubit.dart';
import 'package:admin/features/products/pages/product_form_page.dart';
import 'package:admin/features/products/pages/product_list_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';

import '../../helpers.dart';

Map<String, Object?> productJson(
  int id, {
  bool active = true,
  List<Map<String, Object?>>? variants,
  List<Map<String, Object?>>? images,
}) => {
  'id': id,
  'vendor_id': 1,
  'category_id': null,
  'slug': 'rice-$id',
  'name': {'bn': 'Chal $id', 'en': 'Rice $id'},
  'description': <String, Object>{},
  'unit': 'piece',
  'variable_weight': false,
  'attributes': <String, Object>{},
  'search_keywords': '',
  'seo': <String, Object>{},
  'rating_sum': 0,
  'review_count': 0,
  'is_active': active,
  'created_at': '2026-11-17T04:00:00Z',
  'updated_at': '2026-11-17T04:00:00Z',
  'variants': ?variants,
  'images': ?images,
};

Map<String, Object?> variantJson(int id, {int price = 12000}) => {
  'id': id,
  'product_id': 2,
  'vendor_id': 1,
  'sku': 'SKU-$id',
  'options': {
    'weight': {'bn': '1 kg', 'en': '1 kg'},
  },
  'price': price,
  'compare_at_price': 15000,
  'cost_price': null,
  'nominal_weight_g': null,
  'stock': 3,
  'low_stock_at': 5,
  'is_active': true,
  'sort_order': 0,
};

Map<String, Object?> imageJson(int id) => {
  'id': id,
  'product_id': 2,
  'storage_key': 'shops/1/products/2/$id.jpg',
  'alt': {'bn': 'chobi $id'},
  'width': 800,
  'height': 600,
  'sort_order': id,
};

final categoriesJson = [
  {
    'id': 4,
    'parent_id': null,
    'slug': 'rice',
    'name': {'bn': 'Chal', 'en': 'Rice'},
    'image_key': null,
    'sort_order': 0,
    'is_active': true,
  },
];

Future<void> pumpForm(
  WidgetTester tester,
  FakeApi api, {
  int? id,
  Size size = const Size(1280, 900),
  String locale = 'bn',
}) => pumpPage(
  tester,
  BlocProvider(
    create: (_) => ProductFormCubit(api.catalog, id: id)..load(),
    child: const ProductFormPage(),
  ),
  size: size,
  locale: locale,
);

void main() {
  for (final w in [360.0, 1280.0]) {
    testWidgets('product list ${w.toInt()}px: overflow nai, inactive, page', (
      tester,
    ) async {
      final api = FakeApi({
        'GET /api/admin/products': [
          json(
            200,
            page([productJson(1), productJson(2, active: false)], total: 30),
          ),
        ],
      });
      await pumpPage(
        tester,
        BlocProvider(
          create: (_) => ProductListCubit(api.catalog)..load(),
          child: const ProductListPage(),
        ),
        size: Size(w, 800),
      );
      expect(tester.takeException(), isNull);
      expect(find.text('Chal 1'), findsOneWidget);
      expect(find.byKey(const Key('product.2.inactive')), findsOneWidget);
      expect(find.byKey(const Key('product.1.inactive')), findsNothing);
      expect(find.byKey(const Key('products.new')), findsOneWidget);
      await tester.tap(find.byKey(const Key('products.next')));
      await tester.pumpAndSettle();
      expect(api.calls.last, 'GET /api/admin/products?page=2');
    });
  }

  testWidgets(
    'notun product: khali/bhul slug e POST na; thik hole {bn, en} body',
    (tester) async {
      final api = FakeApi({
        'GET /api/admin/categories': [json(200, categoriesJson)],
        'POST /api/admin/products': [json(201, productJson(9))],
      });
      await pumpForm(tester, api, size: const Size(360, 800));
      expect(tester.takeException(), isNull);
      expect(
        find.byKey(const Key('image.upload')),
        findsNothing,
      ); // save er age chobi na
      await tester.ensureVisible(find.byKey(const Key('product.save')));
      await tester.tap(find.byKey(const Key('product.save')));
      await tester.pumpAndSettle();
      expect(api.calls, ['GET /api/admin/categories']);

      await tester.enterText(
        find.byKey(const Key('product.name_bn')),
        ' Lal chal ',
      );
      await tester.enterText(
        find.byKey(const Key('product.name_en')),
        'Red rice',
      );
      await tester.enterText(find.byKey(const Key('product.slug')), 'Red Rice');
      await tester.ensureVisible(find.byKey(const Key('product.save')));
      await tester.tap(find.byKey(const Key('product.save')));
      await tester.pumpAndSettle();
      expect(api.calls, ['GET /api/admin/categories']);

      await tester.enterText(find.byKey(const Key('product.slug')), 'red-rice');
      await tester.ensureVisible(find.byKey(const Key('product.save')));
      await tester.tap(find.byKey(const Key('product.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['POST /api/admin/products'], {
        'name': {'bn': 'Lal chal', 'en': 'Red rice'},
        'slug': 'red-rice',
        'description': {'bn': '', 'en': ''},
        'category_id': null,
        'unit': 'piece',
        'variable_weight': false,
        'search_keywords': '',
        'is_active': true,
      });
      // save er por chobi ar variant jog kora jay
      expect(find.byKey(const Key('image.upload')), findsOneWidget);
      expect(find.byKey(const Key('variant.add')), findsOneWidget);
    },
  );

  testWidgets('edit: PATCH, slug_taken er lekha', (tester) async {
    final api = FakeApi({
      'GET /api/admin/categories': [json(200, categoriesJson)],
      'GET /api/admin/products/2': [
        json(200, productJson(2, variants: [], images: [])),
      ],
      'PATCH /api/admin/products/2': [err(409, 'slug_taken')],
    });
    await pumpForm(tester, api, id: 2, locale: 'en');
    expect(find.text('Chal 2'), findsWidgets); // form e bhora
    await tester.ensureVisible(find.byKey(const Key('product.save')));
    await tester.tap(find.byKey(const Key('product.save')));
    await tester.pumpAndSettle();
    expect(
      api.bodies['PATCH /api/admin/products/2'],
      containsPair('slug', 'rice-2'),
    );
    expect(find.text('This link is already used'), findsOneWidget);
  });

  testWidgets(
    'variant: notun (taka → poisha, size key), dam er cheye kom compare_at e POST na',
    (tester) async {
      final api = FakeApi({
        'GET /api/admin/categories': [json(200, categoriesJson)],
        'GET /api/admin/products/2': [
          json(200, productJson(2, variants: [], images: [])),
          json(200, productJson(2, variants: [variantJson(5)], images: [])),
        ],
        'POST /api/admin/products/2/variants': [json(201, variantJson(5))],
      });
      await pumpForm(tester, api, id: 2);
      await tester.ensureVisible(find.byKey(const Key('variant.add')));
      await tester.tap(find.byKey(const Key('variant.add')));
      await tester.pumpAndSettle();
      await tester.enterText(find.byKey(const Key('variant.sku')), 'RICE-1');
      await tester.enterText(find.byKey(const Key('variant.label_bn')), '1 kg');
      await tester.enterText(find.byKey(const Key('variant.price')), '120.50');
      await tester.enterText(
        find.byKey(const Key('variant.compare_at')),
        '100',
      );
      await tester.tap(find.byKey(const Key('variant.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['POST /api/admin/products/2/variants'], isNull);

      await tester.enterText(find.byKey(const Key('variant.compare_at')), '');
      await tester.tap(find.byKey(const Key('variant.save')));
      await tester.pumpAndSettle();
      expect(api.bodies['POST /api/admin/products/2/variants'], {
        'sku': 'RICE-1',
        'options': {
          'size': {'bn': '1 kg', 'en': ''},
        },
        'price': 12050,
        'compare_at_price': null,
        'cost_price': null,
        'nominal_weight_g': null,
        'is_active': true,
      });
      expect(find.text('SKU-5'), findsOneWidget); // abar load
    },
  );

  testWidgets('variant edit: ager option key thake, compare_at muchle null', (
    tester,
  ) async {
    final api = FakeApi({
      'GET /api/admin/categories': [json(200, categoriesJson)],
      'GET /api/admin/products/2': [
        json(200, productJson(2, variants: [variantJson(5)], images: [])),
      ],
      'PATCH /api/admin/variants/5': [json(200, variantJson(5))],
    });
    await pumpForm(tester, api, id: 2);
    await tester.ensureVisible(find.byKey(const Key('variant.5')));
    await tester.tap(find.byKey(const Key('variant.5')));
    await tester.pumpAndSettle();
    expect(find.text('150'), findsOneWidget); // 15000 poisha
    await tester.enterText(find.byKey(const Key('variant.compare_at')), '');
    await tester.tap(find.byKey(const Key('variant.save')));
    await tester.pumpAndSettle();
    final body = api.bodies['PATCH /api/admin/variants/5'] as Map;
    expect(body['options'], {
      'weight': {'bn': '1 kg', 'en': '1 kg'},
    });
    expect(body['price'], 12000);
    expect(body['compare_at_price'], isNull);
    expect(body.containsKey('compare_at_price'), isTrue);
  });

  testWidgets('chobi: upload (multipart), kram bodol, delete confirm shoho', (
    tester,
  ) async {
    pickedFile = (bytes: [1, 2, 3], name: 'rice.jpg');
    addTearDown(() => pickedFile = null);
    final api = FakeApi({
      'GET /api/admin/categories': [json(200, categoriesJson)],
      'GET /api/admin/products/2': [
        json(
          200,
          productJson(2, variants: [], images: [imageJson(1), imageJson(3)]),
        ),
      ],
      'POST /api/admin/products/2/images': [json(201, imageJson(7))],
      'PATCH /api/admin/products/2/images/order': [json(204, null)],
      'DELETE /api/admin/images/1': [json(204, null)],
    });
    await pumpForm(tester, api, id: 2);
    await tester.ensureVisible(find.byKey(const Key('image.upload')));
    await tester.tap(find.byKey(const Key('image.upload')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('POST /api/admin/products/2/images'));
    expect(api.bodies['POST /api/admin/products/2/images'], isA<int>());

    await tester.ensureVisible(find.byKey(const Key('image.3.up')));
    await tester.tap(find.byKey(const Key('image.3.up')));
    await tester.pumpAndSettle();
    expect(api.bodies['PATCH /api/admin/products/2/images/order'], {
      'ids': [3, 1],
    });

    await tester.ensureVisible(find.byKey(const Key('image.1.delete')));
    await tester.tap(find.byKey(const Key('image.1.delete')));
    await tester.pumpAndSettle();
    expect(api.calls, isNot(contains('DELETE /api/admin/images/1')));
    await tester.tap(find.byKey(const Key('confirm.ok')));
    await tester.pumpAndSettle();
    expect(api.calls, contains('DELETE /api/admin/images/1'));
  });

  testWidgets('chobi pick bondho korle upload na', (tester) async {
    pickedFile = null;
    final api = FakeApi({
      'GET /api/admin/categories': [json(200, categoriesJson)],
      'GET /api/admin/products/2': [
        json(200, productJson(2, variants: [], images: [])),
      ],
    });
    await pumpForm(tester, api, id: 2);
    await tester.ensureVisible(find.byKey(const Key('image.upload')));
    await tester.tap(find.byKey(const Key('image.upload')));
    await tester.pumpAndSettle();
    expect(api.calls, isNot(contains('POST /api/admin/products/2/images')));
  });
}
