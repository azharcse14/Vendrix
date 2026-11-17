import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Product list er obostha.
sealed class ProductListState {}

/// Load hocche.
final class ProductListLoading extends ProductListState {}

/// Ek page.
final class ProductListLoaded extends ProductListState {
  final Page<AdminProduct> page;
  ProductListLoaded(this.page);
}

/// Load er bhul.
final class ProductListFailed extends ProductListState {
  final ApiError error;
  ProductListFailed(this.error);
}

/// `/products` (11.1): notun age, page (API te khoja nai).
class ProductListCubit extends Cubit<ProductListState> {
  final AdminCatalogRepository repo;
  ProductListCubit(this.repo) : super(ProductListLoading());

  Future<void> load([int page = 1]) async {
    try {
      emit(
        ProductListLoaded(await repo.products(page: page > 1 ? page : null)),
      );
    } on ApiError catch (e) {
      emit(ProductListFailed(e));
    }
  }
}
