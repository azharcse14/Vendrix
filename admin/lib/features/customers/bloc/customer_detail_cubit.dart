import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Customer detail er obostha.
sealed class CustomerDetailState {}

/// Load hocche.
final class CustomerDetailLoading extends CustomerDetailState {}

/// Load er bhul.
final class CustomerDetailFailed extends CustomerDetailState {
  final ApiError error;
  CustomerDetailFailed(this.error);
}

/// Profile ar shesh 50 order; [error] shesh kaj er bhul.
final class CustomerDetailLoaded extends CustomerDetailState {
  final Customer customer;
  final bool busy;
  final ApiError? error;
  CustomerDetailLoaded(this.customer, {this.busy = false, this.error});
}

/// `/customers/:id` (11.7 item 12): profile, order, phone block/unblock.
class CustomerDetailCubit extends Cubit<CustomerDetailState> {
  final AdminCustomerRepository repo;
  final int id;
  CustomerDetailCubit(this.repo, this.id) : super(CustomerDetailLoading());

  Future<void> load() async {
    try {
      emit(CustomerDetailLoaded(await repo.customer(id)));
    } on ApiError catch (e) {
      emit(CustomerDetailFailed(e));
    }
  }

  Future<void> _run(Future<void> Function(String phone) action) async {
    final s = state;
    if (s is! CustomerDetailLoaded || s.busy) return;
    emit(CustomerDetailLoaded(s.customer, busy: true));
    try {
      await action(s.customer.phone);
      emit(CustomerDetailLoaded(await repo.customer(id)));
    } on ApiError catch (e) {
      emit(CustomerDetailLoaded(s.customer, error: e));
    }
  }

  /// Block ([reason] khali = nai); purono order ar session e kichu na (server).
  Future<void> block(String reason) =>
      _run((p) => repo.block(p, reason: reason.isEmpty ? null : reason));

  Future<void> unblock() => _run(repo.unblock);
}
