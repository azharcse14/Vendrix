import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

import '../../../core/browser.dart';

/// Product form er obostha.
sealed class ProductFormState {}

/// Load hocche.
final class ProductFormLoading extends ProductFormState {}

/// Load er bhul.
final class ProductFormFailed extends ProductFormState {
  final ApiError error;
  ProductFormFailed(this.error);
}

/// [product] null = notun (save er age variant/chobi na). [error] shesh kaj er bhul.
final class ProductFormReady extends ProductFormState {
  final AdminProduct? product;
  final List<AdminCategory> categories;
  final bool busy;
  final ApiError? error;
  ProductFormReady(
    this.product,
    this.categories, {
    this.busy = false,
    this.error,
  });
}

/// `/products/new`, `/products/:id` (11.1, 7.7, 11.7 item 9): product, variant, chobi. Protiti kaj er por product
/// abar load (variant/chobi er kram server er).
class ProductFormCubit extends Cubit<ProductFormState> {
  final AdminCatalogRepository repo;
  int? id;
  ProductFormCubit(this.repo, {this.id}) : super(ProductFormLoading());

  Future<void> load() async {
    try {
      final cats = await repo.categories();
      emit(ProductFormReady(id == null ? null : await repo.product(id!), cats));
    } on ApiError catch (e) {
      emit(ProductFormFailed(e));
    }
  }

  /// true = hoyeche. [action] product dile sheta (create), na hole abar load (variant/chobi shoho).
  Future<bool> _run(Future<Object?> Function(int? id) action) async {
    final s = state;
    if (s is! ProductFormReady || s.busy) return false;
    emit(ProductFormReady(s.product, s.categories, busy: true));
    try {
      final p = await action(id);
      emit(
        ProductFormReady(
          p is AdminProduct ? p : await repo.product(id!),
          s.categories,
        ),
      );
      return true;
    } on ApiError catch (e) {
      emit(ProductFormReady(s.product, s.categories, error: e));
      return false;
    }
  }

  /// Notun hole create (tarpor [id] ache), na hole PATCH.
  Future<bool> save(Map<String, Object?> body) => _run((pid) async {
    if (pid != null) return repo.updateProduct(pid, body).then((_) => null);
    final p = await repo.createProduct(body); // notun: variant/chobi nai
    id = p.id;
    return p;
  });

  /// [variantId] null = notun variant.
  Future<bool> saveVariant(int? variantId, Map<String, Object?> body) => _run(
    (pid) => variantId == null
        ? repo.createVariant(pid!, body)
        : repo.updateVariant(variantId, body),
  );

  /// Alt = product er naam (bn, en).
  Future<bool> uploadImage(PickedFile f) => _run((pid) {
    final name = (state as ProductFormReady).product!.name;
    return repo.uploadImage(
      pid!,
      f.bytes,
      f.name,
      altBn: _orNull(name.values['bn']),
      altEn: _orNull(name.values['en']),
    );
  });

  /// [index] er chobi [delta] (−1 upore, +1 niche) ghore; shob id pathay.
  Future<bool> moveImage(int index, int delta) => _run((pid) {
    final ids = [
      for (final i in (state as ProductFormReady).product!.images) i.id,
    ];
    ids.insert(index + delta, ids.removeAt(index));
    return repo.orderImages(pid!, ids);
  });

  Future<bool> deleteImage(int imageId) =>
      _run((_) => repo.deleteImage(imageId));
}

String? _orNull(String? s) => s == null || s.isEmpty ? null : s;
