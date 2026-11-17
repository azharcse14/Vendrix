import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Pack er obostha.
sealed class PackState {}

/// Order load hocche.
final class PackLoading extends PackState {}

/// Pack form: [overLimit] = COD e ojon +10% er beshi, customer er raji note lage (8.11 item 3).
final class PackReady extends PackState {
  final OrderDetail order;
  final Fulfillment fulfillment;
  final bool busy, overLimit;
  final ApiError? error;
  PackReady(
    this.order,
    this.fulfillment, {
    this.busy = false,
    this.overLimit = false,
    this.error,
  });

  /// Gram lage emon item (active ojon-variable; 11.7 13 ii).
  Iterable<AdminItem> get weighed =>
      fulfillment.items.where((i) => i.status == 'active' && i.variableWeight);
}

/// Pack hoye geche.
final class PackDone extends PackState {}

/// Load er bhul (ba fulfillment ei order e nai).
final class PackFailed extends PackState {
  final ApiError error;
  PackFailed(this.error);
}

/// `/packing/:orderId/:fid`: fulfillment er item (order detail theke), ojon diye pack.
class PackCubit extends Cubit<PackState> {
  final AdminOrderRepository repo;
  final int orderId, fulfillmentId;
  PackCubit(this.repo, this.orderId, this.fulfillmentId) : super(PackLoading());

  Future<void> load() async {
    emit(PackLoading());
    try {
      final o = await repo.order(orderId);
      final f = o.fulfillments.where((f) => f.id == fulfillmentId).firstOrNull;
      emit(
        f == null
            ? PackFailed(const ApiError(404, 'not_found'))
            : PackReady(o, f),
      );
    } on ApiError catch (e) {
      emit(PackFailed(e));
    }
  }

  /// [weights] item id → packed gram; `409 weight_over_limit` e [PackReady.overLimit], tarpor [customerOkNote] shoho.
  Future<void> submit(Map<int, int> weights, {String? customerOkNote}) async {
    final s = state;
    if (s is! PackReady || s.busy) return;
    emit(PackReady(s.order, s.fulfillment, busy: true, overLimit: s.overLimit));
    try {
      await repo.pack(fulfillmentId, weights, customerOkNote: customerOkNote);
      emit(PackDone());
    } on ApiError catch (e) {
      emit(
        PackReady(
          s.order,
          s.fulfillment,
          overLimit: e.code == 'weight_over_limit',
          error: e,
        ),
      );
    }
  }
}
