import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Customer screen er obostha.
sealed class CustomersState {}

/// Load hocche.
final class CustomersLoading extends CustomersState {}

/// Load er bhul.
final class CustomersFailed extends CustomersState {
  final ApiError error;
  CustomersFailed(this.error);
}

/// Ek page: [customers] ba (blocked tab e) [blocked]; [error] shesh kaj er bhul.
final class CustomersLoaded extends CustomersState {
  final Page<Customer>? customers;
  final Page<BlockedPhone>? blocked;
  final bool busy;
  final ApiError? error;
  CustomersLoaded({
    this.customers,
    this.blocked,
    this.busy = false,
    this.error,
  });

  /// Ekoi data, notun busy/error.
  CustomersLoaded copy({bool busy = false, ApiError? error}) => CustomersLoaded(
    customers: customers,
    blocked: blocked,
    busy: busy,
    error: error,
  );
}

/// `/customers` (owner/admin, 11.7 item 12): customer khoja (`q` phone ba naam), ar block kora phone (block,
/// unblock; idempotent server e).
class CustomersCubit extends Cubit<CustomersState> {
  final AdminCustomerRepository repo;
  String q = '';
  bool showBlocked = false;
  CustomersCubit(this.repo) : super(CustomersLoading());

  Future<CustomersLoaded> _fetch(int page) async {
    final p = page > 1 ? page : null;
    return showBlocked
        ? CustomersLoaded(blocked: await repo.blockedPhones(page: p))
        : CustomersLoaded(
            customers: await repo.customers(q: q.isEmpty ? null : q, page: p),
          );
  }

  Future<void> load([int page = 1]) async {
    try {
      emit(await _fetch(page));
    } on ApiError catch (e) {
      emit(CustomersFailed(e));
    }
  }

  Future<void> search(String text) {
    q = text.trim();
    return load();
  }

  Future<void> tab({required bool blocked}) {
    if (blocked == showBlocked) return Future.value();
    showBlocked = blocked;
    emit(CustomersLoading());
    return load();
  }

  Future<void> _run(Future<void> Function() action) async {
    final s = state;
    if (s is! CustomersLoaded || s.busy) return;
    emit(s.copy(busy: true));
    try {
      await action();
      emit(await _fetch(1));
    } on ApiError catch (e) {
      emit(s.copy(error: e));
    }
  }

  /// [phone] normalize kora (`+880...`).
  Future<void> block(String phone, String? reason) =>
      _run(() => repo.block(phone, reason: reason));

  Future<void> unblock(String phone) => _run(() => repo.unblock(phone));
}
