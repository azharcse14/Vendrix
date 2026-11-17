import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Dashboard er obostha.
sealed class DashboardState {}

/// Load hocche.
final class DashboardLoading extends DashboardState {}

/// Ajker count (11.7 13 viii).
final class DashboardLoaded extends DashboardState {
  final Dashboard data;
  DashboardLoaded(this.data);
}

/// Load er bhul.
final class DashboardFailed extends DashboardState {
  final ApiError error;
  DashboardFailed(this.error);
}

/// `/`: `GET /api/admin/dashboard`.
class DashboardCubit extends Cubit<DashboardState> {
  final DashboardRepository repo;
  DashboardCubit(this.repo) : super(DashboardLoading());

  Future<void> load() async {
    emit(DashboardLoading());
    try {
      emit(DashboardLoaded(await repo.dashboard()));
    } on ApiError catch (e) {
      emit(DashboardFailed(e));
    }
  }
}
