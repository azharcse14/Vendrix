import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Pick list er obostha.
sealed class PickListState {}

/// Load hocche.
final class PickListLoading extends PickListState {}

/// Ajker pick list; [error] print er bhul.
final class PickListLoaded extends PickListState {
  final PickList list;
  final ApiError? error;
  PickListLoaded(this.list, {this.error});
}

/// Load er bhul.
final class PickListFailed extends PickListState {
  final ApiError error;
  PickListFailed(this.error);
}

/// `/packing` (8.11 item 1, 11.4): ajker pick list ar tar print.
class PickListCubit extends Cubit<PickListState> {
  final AdminOrderRepository repo;
  final void Function(Future<String> url) openTab;
  PickListCubit(this.repo, this.openTab) : super(PickListLoading());

  Future<void> load() async {
    emit(PickListLoading());
    try {
      emit(PickListLoaded(await repo.pickList()));
    } on ApiError catch (e) {
      emit(PickListFailed(e));
    }
  }

  /// Pick list print notun tab e (11.6), list er din.
  void print() {
    final s = state;
    if (s is! PickListLoaded) return;
    final url = repo
        .printLink('pick_list', date: s.list.date)
        .then((r) => r.url);
    openTab(url);
    url.catchError((Object e) {
      if (e is ApiError && state is PickListLoaded) {
        emit(PickListLoaded(s.list, error: e));
      }
      return '';
    });
  }
}
