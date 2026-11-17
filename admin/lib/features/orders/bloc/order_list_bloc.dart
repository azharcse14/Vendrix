import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Order list er tab (11.4, 11.7 13 vii): `all`, duita highlight tab, baki `display_status`.
const orderTabs = [
  'all',
  'awaiting_confirm',
  'deadline_soon',
  'pending',
  'confirmed',
  'packed',
  'shipped',
  'partial',
  'delivered',
  'returned',
  'cancelled',
];

const _tabParams = {'awaiting_confirm', 'deadline_soon'};

/// Order list er filter.
class OrderFilter {
  final String tab, q;
  final bool risk;
  const OrderFilter({this.tab = 'all', this.q = '', this.risk = false});

  OrderFilter copyWith({String? tab, String? q, bool? risk}) => OrderFilter(
    tab: tab ?? this.tab,
    q: q ?? this.q,
    risk: risk ?? this.risk,
  );
}

/// Order list er event.
sealed class OrderListEvent {}

/// Prothom load (ba refresh).
final class OrderListStarted extends OrderListEvent {}

/// Tab bodol; page 1.
final class OrderTabSelected extends OrderListEvent {
  final String tab;
  OrderTabSelected(this.tab);
}

/// Number ba phone diye khoja; khali = shob.
final class OrderSearchSubmitted extends OrderListEvent {
  final String q;
  OrderSearchSubmitted(this.q);
}

/// Shudhu risk flag er order, chalu/bondho.
final class OrderRiskToggled extends OrderListEvent {}

/// Onno page (OFFSET paging, tai "aro" na: notun order ele row shore jay).
final class OrderPageRequested extends OrderListEvent {
  final int page;
  OrderPageRequested(this.page);
}

/// Order list er obostha; protitite [filter].
sealed class OrderListState {
  final OrderFilter filter;
  const OrderListState(this.filter);
}

/// Load hocche.
final class OrderListLoading extends OrderListState {
  const OrderListLoading(super.filter);
}

/// Ek page.
final class OrderListLoaded extends OrderListState {
  final Page<AdminOrder> page;
  const OrderListLoaded(super.filter, this.page);
}

/// Load er bhul.
final class OrderListFailed extends OrderListState {
  final ApiError error;
  const OrderListFailed(super.filter, this.error);
}

/// `/orders` (11.4): tab, khoja, risk, page.
class OrderListBloc extends Bloc<OrderListEvent, OrderListState> {
  final AdminOrderRepository repo;

  OrderListBloc(this.repo, {String tab = 'all'})
    : super(OrderListLoading(OrderFilter(tab: tab))) {
    on<OrderListStarted>((_, emit) => _load(emit, state.filter, 1));
    on<OrderTabSelected>(
      (e, emit) => _load(emit, state.filter.copyWith(tab: e.tab), 1),
    );
    on<OrderSearchSubmitted>(
      (e, emit) => _load(emit, state.filter.copyWith(q: e.q.trim()), 1),
    );
    on<OrderRiskToggled>(
      (_, emit) =>
          _load(emit, state.filter.copyWith(risk: !state.filter.risk), 1),
    );
    on<OrderPageRequested>((e, emit) => _load(emit, state.filter, e.page));
  }

  Future<void> _load(
    Emitter<OrderListState> emit,
    OrderFilter f,
    int page,
  ) async {
    emit(OrderListLoading(f));
    try {
      final p = await repo.orders(
        status: f.tab == 'all' || _tabParams.contains(f.tab) ? null : f.tab,
        tab: _tabParams.contains(f.tab) ? f.tab : null,
        risk: f.risk ? true : null,
        q: f.q.isEmpty ? null : f.q,
        page: page,
      );
      emit(OrderListLoaded(f, p));
    } on ApiError catch (e) {
      emit(OrderListFailed(f, e));
    }
  }
}
