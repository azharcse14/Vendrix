import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Order detail er obostha.
sealed class OrderDetailState {}

/// Prothom load.
final class OrderDetailLoading extends OrderDetailState {}

/// Order; [busy] kaj cholche, [error] shesh kaj er bhul, [errorKey] thakle error er bodole oi lekha (staff er
/// `403` er karon).
final class OrderDetailLoaded extends OrderDetailState {
  final OrderDetail order;
  final bool busy;
  final ApiError? error;
  final String? errorKey;
  OrderDetailLoaded(this.order, {this.busy = false, this.error, this.errorKey});
}

/// Load er bhul.
final class OrderDetailFailed extends OrderDetailState {
  final ApiError error;
  OrderDetailFailed(this.error);
}

/// `/orders/:id` (11.4): confirm/cancel, fulfillment er kaj, note, print. Protiti kaj er por abar load
/// (cancel cascade, display_status server e).
class OrderDetailCubit extends Cubit<OrderDetailState> {
  final AdminOrderRepository repo;
  final void Function(Future<String> url) openTab;
  final int id;

  OrderDetailCubit(this.repo, this.openTab, this.id)
    : super(OrderDetailLoading());

  Future<void> load() async {
    try {
      emit(OrderDetailLoaded(await repo.order(id)));
    } on ApiError catch (e) {
      emit(OrderDetailFailed(e));
    }
  }

  Future<void> _run(
    Future<void> Function() action, {
    String? forbiddenKey,
  }) async {
    final s = state;
    if (s is! OrderDetailLoaded || s.busy) return;
    emit(OrderDetailLoaded(s.order, busy: true));
    try {
      await action();
      emit(OrderDetailLoaded(await repo.order(id)));
    } on ApiError catch (e) {
      emit(
        OrderDetailLoaded(
          s.order,
          error: e,
          errorKey: e.code == 'forbidden' ? forbiddenKey : null,
        ),
      );
    }
  }

  /// Order edit (11.7 item 7): item id → notun qty (shudhu kom, `0` = bad), note lagbe.
  Future<void> editItems(Map<int, int> quantities, String note) => _run(
    () => repo.editItems(id, quantities, note),
    forbiddenKey: 'admin.edit.forbidden',
  );

  /// Return (11.7 item 6): [restockItemIds] stock e ferot; taka newa hole refund (staff e `403`).
  Future<void> returnFulfillment(
    int fid,
    List<int> restockItemIds,
    String note,
  ) => _run(
    () => repo.transitionFulfillment(
      fid,
      'returned',
      note: note,
      restockItemIds: restockItemIds,
    ),
    forbiddenKey: 'admin.return.forbidden',
  );

  /// Order `confirmed` ba `cancelled` ([note] cancel er karon).
  Future<void> transition(String to, {String? note}) =>
      _run(() => repo.transition(id, to, note: note));

  /// Fulfillment `open` (undo, note lage) ba `cancelled`.
  Future<void> transitionFulfillment(int fid, String to, {String? note}) =>
      _run(() => repo.transitionFulfillment(fid, to, note: note));

  /// Staff er note (`note` ba `call`).
  Future<void> addNote(String note, {String kind = 'note'}) =>
      _run(() => repo.addNote(id, note, kind: kind));

  /// Packing slip ba invoice notun tab e (11.6).
  void print(String kind) {
    final url = repo.printLink(kind, orderIds: [id]).then((r) => r.url);
    openTab(url);
    url.catchError((Object e) {
      final s = state;
      if (e is ApiError && s is OrderDetailLoaded) {
        emit(OrderDetailLoaded(s.order, error: e));
      }
      return '';
    });
  }
}
