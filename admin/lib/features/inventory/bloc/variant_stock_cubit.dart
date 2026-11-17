import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Variant er stock screen er obostha.
sealed class VariantStockState {}

/// Load hocche.
final class VariantStockLoading extends VariantStockState {}

/// Load er bhul.
final class VariantStockFailed extends VariantStockState {
  final ApiError error;
  VariantStockFailed(this.error);
}

/// Batch (FIFO kram) ar movement (notun age); [error] shesh kaj er bhul.
final class VariantStockLoaded extends VariantStockState {
  final List<Batch> batches;
  final List<Movement> movements;
  final bool busy;
  final ApiError? error;
  VariantStockLoaded(
    this.batches,
    this.movements, {
    this.busy = false,
    this.error,
  });
}

/// `/inventory/:variantId` (8.12, 11.7 item 2): batch, movement, stock entry (batch), komano (adjust/wastage).
class VariantStockCubit extends Cubit<VariantStockState> {
  final InventoryRepository repo;
  final int variantId;
  VariantStockCubit(this.repo, this.variantId) : super(VariantStockLoading());

  // ponytail: prothom page (batch ar movement 20 ta); beshi hole paging.
  Future<VariantStockLoaded> _fetch() async => VariantStockLoaded(
    (await repo.batches(variantId)).items,
    (await repo.movements(variantId: variantId)).items,
  );

  Future<void> load() async {
    try {
      emit(await _fetch());
    } on ApiError catch (e) {
      emit(VariantStockFailed(e));
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    final s = state;
    if (s is! VariantStockLoaded || s.busy) return;
    emit(VariantStockLoaded(s.batches, s.movements, busy: true));
    try {
      await action();
      emit(await _fetch());
    } on ApiError catch (e) {
      emit(VariantStockLoaded(s.batches, s.movements, error: e));
    }
  }

  /// Stock barano shudhu batch diye (8.12).
  Future<void> addBatch(
    int quantity, {
    int? costPrice,
    String? batchCode,
    String? harvestDate,
    String? expiryDate,
  }) => _run(
    () => repo.createBatch(
      variantId,
      quantity,
      costPrice: costPrice,
      batchCode: batchCode,
      harvestDate: harvestDate,
      expiryDate: expiryDate,
    ),
  );

  /// Shudhu komano: [quantity] > 0, API te `change = -quantity`; [reason] `wastage` ba `adjust`, note lagbe.
  Future<void> reduce(int quantity, String reason, String note) =>
      _run(() => repo.adjust(variantId, -quantity, reason, note));
}
