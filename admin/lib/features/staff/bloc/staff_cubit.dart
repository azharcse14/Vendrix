import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Staff screen er obostha.
sealed class StaffState {}

/// Load hocche.
final class StaffLoading extends StaffState {}

/// Load er bhul.
final class StaffFailed extends StaffState {
  final ApiError error;
  StaffFailed(this.error);
}

/// Staff list; [error] shesh kaj er bhul. Temporary password state e rakha hoy na (ekbar dialog e).
final class StaffLoaded extends StaffState {
  final List<Staff> staff;
  final bool busy;
  final ApiError? error;
  StaffLoaded(this.staff, {this.busy = false, this.error});
}

/// `/staff` (owner, 11.7 item 11): create, role/active, reset.
class StaffCubit extends Cubit<StaffState> {
  final StaffRepository repo;
  StaffCubit(this.repo) : super(StaffLoading());

  // ponytail: prothom page (20); dokan e eto staff hoy na, beshi hole paging.
  Future<StaffLoaded> _fetch() async => StaffLoaded((await repo.staff()).items);

  Future<void> load() async {
    try {
      emit(await _fetch());
    } on ApiError catch (e) {
      emit(StaffFailed(e));
    }
  }

  /// Kaj er uttor (temporary password ba null); bhul hole null ar state e error.
  Future<T?> _run<T>(Future<T> Function() action) async {
    final s = state;
    if (s is! StaffLoaded || s.busy) return null;
    emit(StaffLoaded(s.staff, busy: true));
    try {
      final r = await action();
      emit(await _fetch());
      return r;
    } on ApiError catch (e) {
      emit(StaffLoaded(s.staff, error: e));
      return null;
    }
  }

  /// Notun staff → temporary password (ekbar dekhano).
  Future<String?> create(String email, String name, String role) =>
      _run<String?>(
        () async => (await repo.create(email, name, role)).tempPassword,
      );

  /// Notun temporary password; TOTP muche, staff abar setup kore (8.3).
  Future<String?> reset(int id) => _run(() => repo.reset(id));

  /// Shudhu deya field (role ba active bodlale oi staff er session bondho, server e).
  Future<void> update(int id, {String? role, bool? isActive}) =>
      _run(() => repo.update(id, role: role, isActive: isActive));
}
