import 'package:admin/features/categories/bloc/categories_cubit.dart';
import 'package:admin/features/categories/pages/categories_page.dart';
import 'package:flutter/material.dart';
import 'package:flutter_bloc/flutter_bloc.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:shared/shared.dart';

import '../../helpers.dart';

Map<String, Object?> cat(int id, {int? parent, bool active = true}) => {
  'id': id,
  'parent_id': parent,
  'slug': 'cat-$id',
  'name': {'bn': 'Cat $id', 'en': 'Category $id'},
  'image_key': null,
  'sort_order': 0,
  'is_active': active,
};

// API sort_order, id kram e dey; child parent er age thakte pare.
final tree = [cat(3, parent: 1), cat(1), cat(2), cat(4, parent: 3)];

Future<FakeApi> pump(
  WidgetTester tester, {
  Map<String, List<dynamic>> extra = const {},
  Size size = const Size(1280, 900),
  String locale = 'bn',
}) async {
  final api = FakeApi({
    'GET /api/admin/categories': [json(200, tree)],
    for (final e in extra.entries) e.key: [...e.value.cast()],
  });
  await pumpPage(
    tester,
    BlocProvider(
      create: (_) => CategoriesCubit(api.catalog)..load(),
      child: const CategoriesPage(),
    ),
    size: size,
    locale: locale,
  );
  return api;
}

void main() {
  test('flatten: parent er niche child, depth shoho', () {
    final rows = categoryTree([
      for (final c in tree) AdminCategory.fromJson(c),
    ]);
    expect(rows.map((r) => (r.category.id, r.depth)), [
      (1, 0),
      (3, 1),
      (4, 2),
      (2, 0),
    ]);
  });

  for (final w in [360.0, 1280.0]) {
    testWidgets('tree ${w.toInt()}px: overflow nai, indent', (tester) async {
      await pump(tester, size: Size(w, 800));
      expect(tester.takeException(), isNull);
      final y = [
        for (final id in [1, 3, 4, 2])
          tester.getTopLeft(find.byKey(Key('category.$id'))).dy,
      ];
      expect(y, orderedEquals([...y]..sort()));
      expect(
        tester.getTopLeft(find.text('Cat 3')).dx,
        greaterThan(tester.getTopLeft(find.text('Cat 1')).dx),
      );
    });
  }

  testWidgets('notun category: khali naam e POST na; thik hole body', (
    tester,
  ) async {
    final api = await pump(
      tester,
      extra: {
        'POST /api/admin/categories': [json(201, cat(9))],
      },
    );
    await tester.tap(find.byKey(const Key('categories.new')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('category.save')));
    await tester.pumpAndSettle();
    expect(api.calls, ['GET /api/admin/categories']);
    await tester.enterText(find.byKey(const Key('category.name_bn')), 'Dal');
    await tester.enterText(find.byKey(const Key('category.slug')), 'dal');
    await tester.tap(find.byKey(const Key('category.save')));
    await tester.pumpAndSettle();
    expect(api.bodies['POST /api/admin/categories'], {
      'name': {'bn': 'Dal', 'en': ''},
      'slug': 'dal',
      'parent_id': null,
      'sort_order': 0,
      'is_active': true,
    });
    expect(api.calls.last, 'GET /api/admin/categories'); // abar load
  });

  testWidgets('edit: parent list e nije ar nijer child nai; PATCH', (
    tester,
  ) async {
    final api = await pump(
      tester,
      extra: {
        'PATCH /api/admin/categories/3': [json(200, cat(3))],
      },
    );
    await tester.tap(find.byKey(const Key('category.3.edit')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('category.parent')));
    await tester.pumpAndSettle();
    // menu khola: parent.1 button e (bechhe newa) ar menu te
    expect(find.byKey(const Key('parent.1')), findsNWidgets(2));
    expect(find.byKey(const Key('parent.2')), findsOneWidget);
    expect(find.byKey(const Key('parent.3')), findsNothing);
    expect(find.byKey(const Key('parent.4')), findsNothing);
    await tester.tap(find.byKey(const Key('parent.2')));
    await tester.pumpAndSettle();
    await tester.tap(find.byKey(const Key('category.save')));
    await tester.pumpAndSettle();
    expect(
      api.bodies['PATCH /api/admin/categories/3'],
      containsPair('parent_id', 2),
    );
  });

  testWidgets('delete: confirm er por; category_not_empty er lekha', (
    tester,
  ) async {
    final api = await pump(
      tester,
      locale: 'en',
      extra: {
        'DELETE /api/admin/categories/1': [err(409, 'category_not_empty')],
      },
    );
    await tester.tap(find.byKey(const Key('category.1.delete')));
    await tester.pumpAndSettle();
    expect(api.calls, ['GET /api/admin/categories']);
    await tester.tap(find.byKey(const Key('confirm.ok')));
    await tester.pumpAndSettle();
    expect(api.calls.last, 'DELETE /api/admin/categories/1');
    expect(find.text('This category is not empty'), findsOneWidget);
    expect(find.byKey(const Key('category.1')), findsOneWidget);
  });
}
