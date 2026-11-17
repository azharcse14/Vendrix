import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Zone screen er obostha.
sealed class ZonesState {}

/// Load hocche.
final class ZonesLoading extends ZonesState {}

/// Load er bhul.
final class ZonesFailed extends ZonesState {
  final ApiError error;
  ZonesFailed(this.error);
}

/// Ek page (inactive o); [error] shesh kaj er bhul.
final class ZonesLoaded extends ZonesState {
  final Page<Zone> page;
  final bool busy;
  final ApiError? error;
  ZonesLoaded(this.page, {this.busy = false, this.error});
}

/// `/zones` (owner/admin, 11.7 item 13 vi): list, notun, edit (delete na, `is_active`).
class ZonesCubit extends Cubit<ZonesState> {
  final AdminZoneRepository repo;
  ZonesCubit(this.repo) : super(ZonesLoading());

  Future<void> load([int page = 1]) async {
    try {
      emit(ZonesLoaded(await repo.zones(page: page > 1 ? page : null)));
    } on ApiError catch (e) {
      emit(ZonesFailed(e));
    }
  }

  /// [id] null = notun; tarpor oi page abar load.
  Future<void> save(int? id, Map<String, Object?> body) async {
    final s = state;
    if (s is! ZonesLoaded || s.busy) return;
    emit(ZonesLoaded(s.page, busy: true));
    try {
      await (id == null ? repo.create(body) : repo.update(id, body));
      await load(s.page.page);
    } on ApiError catch (e) {
      emit(ZonesLoaded(s.page, error: e));
    }
  }
}
