import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Audit screen er obostha.
sealed class AuditState {}

/// Load hocche.
final class AuditLoading extends AuditState {}

/// Load er bhul.
final class AuditFailed extends AuditState {
  final ApiError error;
  AuditFailed(this.error);
}

/// Ek page (notun age).
final class AuditLoaded extends AuditState {
  final Page<AuditEntry> page;
  AuditLoaded(this.page);
}

/// Audit filter (khali = nai); `from`/`to` shop er din `YYYY-MM-DD` (`to` shoho).
typedef AuditFilter = ({
  String? action,
  int? staffId,
  String? from,
  String? to,
});

/// `/audit` (owner/admin, read-only, 11.7 item 11): filter `action`, `staff_id`, `from`, `to`.
class AuditCubit extends Cubit<AuditState> {
  final AuditRepository repo;
  AuditFilter filter = (action: null, staffId: null, from: null, to: null);
  AuditCubit(this.repo) : super(AuditLoading());

  Future<void> load([int page = 1]) async {
    try {
      emit(
        AuditLoaded(
          await repo.logs(
            action: filter.action,
            staffId: filter.staffId,
            from: filter.from,
            to: filter.to,
            page: page > 1 ? page : null,
          ),
        ),
      );
    } on ApiError catch (e) {
      emit(AuditFailed(e));
    }
  }

  Future<void> apply(AuditFilter f) {
    filter = f;
    return load();
  }
}
