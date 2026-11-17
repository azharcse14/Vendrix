import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Tree er ek line: category ar gobhirota (root 0).
typedef CategoryRow = ({AdminCategory category, int depth});

/// Flat list → tree kram (parent er thik niche child, API er kram rekhe). Parent na pele root; cycle e atke na.
List<CategoryRow> categoryTree(List<AdminCategory> all) {
  final ids = {for (final c in all) c.id};
  final children = <int?, List<AdminCategory>>{};
  for (final c in all) {
    children
        .putIfAbsent(ids.contains(c.parentId) ? c.parentId : null, () => [])
        .add(c);
  }
  final out = <CategoryRow>[];
  final seen = <int>{};
  void walk(int? parent, int depth) {
    for (final c in children[parent] ?? const <AdminCategory>[]) {
      if (!seen.add(c.id)) continue;
      out.add((category: c, depth: depth));
      walk(c.id, depth + 1);
    }
  }

  walk(null, 0);
  return out;
}

/// [id] ar tar shob child (parent bechhe newar list theke bad).
Set<int> subtree(List<AdminCategory> all, int id) {
  final out = {id};
  for (var grew = true; grew;) {
    grew = false;
    for (final c in all) {
      if (out.contains(c.parentId) && out.add(c.id)) grew = true;
    }
  }
  return out;
}

/// Category screen er obostha.
sealed class CategoriesState {}

/// Load hocche.
final class CategoriesLoading extends CategoriesState {}

/// Load er bhul.
final class CategoriesFailed extends CategoriesState {
  final ApiError error;
  CategoriesFailed(this.error);
}

/// Shob category (inactive o); [error] shesh kaj er bhul.
final class CategoriesLoaded extends CategoriesState {
  final List<AdminCategory> all;
  final bool busy;
  final ApiError? error;
  CategoriesLoaded(this.all, {this.busy = false, this.error});
}

/// `/categories` (11.1, 7.7): tree, notun, edit, delete.
class CategoriesCubit extends Cubit<CategoriesState> {
  final AdminCatalogRepository repo;
  CategoriesCubit(this.repo) : super(CategoriesLoading());

  Future<void> load() async {
    try {
      emit(CategoriesLoaded(await repo.categories()));
    } on ApiError catch (e) {
      emit(CategoriesFailed(e));
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    final s = state;
    if (s is! CategoriesLoaded || s.busy) return;
    emit(CategoriesLoaded(s.all, busy: true));
    try {
      await action();
      emit(CategoriesLoaded(await repo.categories()));
    } on ApiError catch (e) {
      emit(CategoriesLoaded(s.all, error: e));
    }
  }

  /// [id] null = notun.
  Future<void> save(int? id, Map<String, Object?> body) => _run(
    () =>
        id == null ? repo.createCategory(body) : repo.updateCategory(id, body),
  );

  /// Product ba child thakle `409 category_not_empty`.
  Future<void> delete(int id) => _run(() => repo.deleteCategory(id));
}
