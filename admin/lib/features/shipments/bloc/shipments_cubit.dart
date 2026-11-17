import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Carrier (6.7, 11.7 item 5), manual.
const carriers = ['own_rider', 'steadfast', 'pathao', 'redx', 'other'];

/// Shipment screen er obostha.
sealed class ShipmentsState {}

/// Load hocche.
final class ShipmentsLoading extends ShipmentsState {}

/// [toShip]: packed, ba shipped kintu active shipment nai (ager parcel ferot/bhul, abar pathano);
/// [onTheWay]: active shipment shoho. [error] shesh kaj er bhul.
final class ShipmentsLoaded extends ShipmentsState {
  final List<FulfillmentRow> toShip, onTheWay;
  final bool busy;
  final ApiError? error;
  ShipmentsLoaded(this.toShip, this.onTheWay, {this.busy = false, this.error});
}

/// Load er bhul.
final class ShipmentsFailed extends ShipmentsState {
  final ApiError error;
  ShipmentsFailed(this.error);
}

/// `/shipments` (11.1, 11.7 item 5): shipment banano, delivered, parcel ferot, booking bhul.
class ShipmentsCubit extends Cubit<ShipmentsState> {
  final AdminOrderRepository repo;
  ShipmentsCubit(this.repo) : super(ShipmentsLoading());

  // ponytail: protiti list er prothom 100 (7.1 max); beshi hole paging.
  Future<ShipmentsLoaded> _fetch() async {
    final packed = await repo.fulfillments(status: 'packed', limit: 100);
    final shipped = await repo.fulfillments(status: 'shipped', limit: 100);
    return ShipmentsLoaded([
      ...packed.items,
      ...shipped.items.where((r) => r.shipment == null),
    ], shipped.items.where((r) => r.shipment != null).toList());
  }

  Future<void> load() async {
    try {
      emit(await _fetch());
    } on ApiError catch (e) {
      emit(ShipmentsFailed(e));
    }
  }

  Future<void> _run(Future<void> Function() action) async {
    final s = state;
    if (s is! ShipmentsLoaded || s.busy) return;
    emit(ShipmentsLoaded(s.toShip, s.onTheWay, busy: true));
    try {
      await action();
      emit(await _fetch());
    } on ApiError catch (e) {
      emit(ShipmentsLoaded(s.toShip, s.onTheWay, error: e));
    }
  }

  /// Shipment banano (`packed → shipped`); khali tracking pathano hoy na.
  Future<void> ship(int fulfillmentId, String carrier, {String tracking = ''}) {
    final t = tracking.trim();
    return _run(
      () =>
          repo.ship(fulfillmentId, carrier, trackingCode: t.isEmpty ? null : t),
    );
  }

  /// `delivered`, `returned` (parcel ferot) ba `cancelled` (booking bhul).
  Future<void> updateShipment(int id, String status) =>
      _run(() => repo.updateShipment(id, status: status));
}
