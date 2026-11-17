import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Inventory list er obostha.
sealed class InventoryListState {}

/// Load hocche.
final class InventoryListLoading extends InventoryListState {}

/// Ek page.
final class InventoryListLoaded extends InventoryListState {
  final Page<InventoryVariant> page;
  InventoryListLoaded(this.page);
}

/// Load er bhul.
final class InventoryListFailed extends InventoryListState {
  final ApiError error;
  InventoryListFailed(this.error);
}

/// `/inventory` (11.7 item 1): variant khoja (`q` naam ba SKU), `low` = stock ≤ shima, page.
class InventoryListCubit extends Cubit<InventoryListState> {
  final InventoryRepository repo;
  String q = '';
  bool low = false;
  InventoryListCubit(this.repo, {this.low = false})
    : super(InventoryListLoading());

  Future<void> load([int page = 1]) async {
    try {
      emit(
        InventoryListLoaded(
          await repo.variants(
            q: q.isEmpty ? null : q,
            low: low ? true : null,
            page: page > 1 ? page : null,
          ),
        ),
      );
    } on ApiError catch (e) {
      emit(InventoryListFailed(e));
    }
  }

  Future<void> search(String text) {
    q = text.trim();
    return load();
  }

  Future<void> toggleLow() {
    low = !low;
    return load();
  }
}
