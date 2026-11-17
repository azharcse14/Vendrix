import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Coupon screen er obostha.
sealed class CouponsState {}

/// Load hocche.
final class CouponsLoading extends CouponsState {}

/// Load er bhul.
final class CouponsFailed extends CouponsState {
  final ApiError error;
  CouponsFailed(this.error);
}

/// Ek page (notun age); [error] shesh kaj er bhul.
final class CouponsLoaded extends CouponsState {
  final Page<Coupon> page;
  final bool busy;
  final ApiError? error;
  CouponsLoaded(this.page, {this.busy = false, this.error});
}

/// `/coupons` (owner/admin, 11.7 item 13 v): list, notun, edit, active (delete na).
class CouponsCubit extends Cubit<CouponsState> {
  final AdminCouponRepository repo;
  CouponsCubit(this.repo) : super(CouponsLoading());

  Future<void> load([int page = 1]) async {
    try {
      emit(CouponsLoaded(await repo.coupons(page: page > 1 ? page : null)));
    } on ApiError catch (e) {
      emit(CouponsFailed(e));
    }
  }

  /// [id] null = notun; tarpor oi page abar load.
  Future<void> save(int? id, Map<String, Object?> body) async {
    final s = state;
    if (s is! CouponsLoaded || s.busy) return;
    emit(CouponsLoaded(s.page, busy: true));
    try {
      await (id == null ? repo.create(body) : repo.update(id, body));
      await load(s.page.page);
    } on ApiError catch (e) {
      emit(CouponsLoaded(s.page, error: e));
    }
  }

  /// Shudhu `is_active` (bebohar holeo chole).
  Future<void> setActive(int id, bool active) =>
      save(id, {'is_active': active});
}
