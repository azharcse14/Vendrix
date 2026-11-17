import 'package:bloc/bloc.dart';
import 'package:shared/shared.dart';

/// Login er event (8.3).
sealed class AuthEvent {}

/// App shuru: `GET /api/admin/me` diye session ache kina.
final class AuthStarted extends AuthEvent {}

/// Shop slug, email, password.
final class LoginSubmitted extends AuthEvent {
  final String shop, email, password;
  LoginSubmitted(this.shop, this.email, this.password);
}

/// TOTP code, ba [recovery] hole recovery code.
final class MfaSubmitted extends AuthEvent {
  final String code;
  final bool recovery;
  MfaSubmitted(this.code, {this.recovery = false});
}

/// Setup e password bodol.
final class PasswordSubmitted extends AuthEvent {
  final String old, new_;
  PasswordSubmitted(this.old, this.new_);
}

/// Setup e authenticator app er code.
final class TotpEnableSubmitted extends AuthEvent {
  final String code;
  TotpEnableSubmitted(this.code);
}

/// Recovery code rakha hoyeche.
final class RecoveryCodesSaved extends AuthEvent {}

/// Logout (ba TOTP theke login e ferot).
final class LogoutRequested extends AuthEvent {}

/// Login er obostha; router er redirect eta dekhe (11.3).
sealed class AuthState {}

/// Shuru te, `me` er uttor er age.
final class AuthUnknown extends AuthState {}

/// Login lage; [error] shesh cheshtar bhul.
final class Unauthenticated extends AuthState {
  final ApiError? error;
  Unauthenticated({this.error});
}

/// Password thik, TOTP ba recovery code lage.
final class TotpRequired extends AuthState {
  final String tempToken;
  final ApiError? error;
  TotpRequired(this.tempToken, {this.error});

  @override
  String toString() => 'TotpRequired'; // temp_token log e na
}

/// `setup_pending` er dhap.
enum SetupStep { password, totp, recoveryCodes }

/// Prothom login ba reset er por setup (8.3 step 4).
final class SetupRequired extends AuthState {
  final SetupStep step;
  final String? secret, otpauthUrl;
  final List<String> recoveryCodes;
  final ApiError? error;
  SetupRequired(
    this.step, {
    this.secret,
    this.otpauthUrl,
    this.recoveryCodes = const [],
    this.error,
  });

  @override
  String toString() => 'SetupRequired($step)'; // secret ar code log e na
}

/// Active session.
final class Authenticated extends AuthState {
  final StaffProfile staff;
  Authenticated(this.staff);
}

/// App-wide login (11.3). Server e session `__Host-asid` cookie; ekhane token nai.
class AuthBloc extends Bloc<AuthEvent, AuthState> {
  final AdminAuthRepository repo;

  AuthBloc(this.repo) : super(AuthUnknown()) {
    on<AuthStarted>((_, emit) => _me(emit, quiet: true));
    on<LoginSubmitted>((e, emit) async {
      try {
        final r = await repo.login(e.shop, e.email, e.password);
        if (r.totpRequired) return emit(TotpRequired(r.tempToken!));
        r.setupRequired ? await _setup(emit) : await _me(emit);
      } on ApiError catch (err) {
        emit(Unauthenticated(error: err));
      }
    });
    on<MfaSubmitted>((e, emit) async {
      final token = (state as TotpRequired).tempToken;
      try {
        final setup = e.recovery
            ? await repo.recovery(token, e.code)
            : await repo.totp(token, e.code);
        setup ? await _setup(emit) : await _me(emit);
      } on ApiError catch (err) {
        // bhul code e temp_token thake; expire ba lock e abar login
        emit(
          err.code == 'mfa_invalid' || err.code == 'validation_failed'
              ? TotpRequired(token, error: err)
              : Unauthenticated(error: err),
        );
      }
    });
    on<PasswordSubmitted>((e, emit) async {
      try {
        await repo.changePassword(e.old, e.new_);
      } on ApiError catch (err) {
        return emit(_setupError(err, SetupRequired(SetupStep.password)));
      }
      await _me(emit); // TOTP live chhilo hole session ekhon active
    });
    on<TotpEnableSubmitted>((e, emit) async {
      final s = state as SetupRequired;
      try {
        final codes = await repo.totpEnable(e.code);
        emit(SetupRequired(SetupStep.recoveryCodes, recoveryCodes: codes));
      } on ApiError catch (err) {
        emit(
          _setupError(
            err,
            SetupRequired(
              SetupStep.totp,
              secret: s.secret,
              otpauthUrl: s.otpauthUrl,
            ),
          ),
        );
      }
    });
    on<RecoveryCodesSaved>((_, emit) => _me(emit));
    on<LogoutRequested>((_, emit) async {
      try {
        await repo.logout();
      } on ApiError {
        // session already nai ba network: local e logout-i
      }
      emit(Unauthenticated());
    });
  }

  // Setup er bhul: session gele login e, na hole oi dhap e error shoho.
  AuthState _setupError(ApiError err, SetupRequired same) => err.status == 401
      ? Unauthenticated(error: err)
      : SetupRequired(
          same.step,
          secret: same.secret,
          otpauthUrl: same.otpauthUrl,
          error: err,
        );

  Future<void> _me(Emitter<AuthState> emit, {bool quiet = false}) async {
    try {
      emit(Authenticated(await repo.me()));
    } on ApiError catch (err) {
      if (err.code == 'setup_required') return _setup(emit);
      // shuru te session na thaka bhul na
      emit(Unauthenticated(error: quiet && err.status == 401 ? null : err));
    }
  }

  // Password age lage kina server bole (`password_change_required`); totp/setup ekbar e notun pending secret.
  Future<void> _setup(Emitter<AuthState> emit) async {
    try {
      final r = await repo.totpSetup();
      emit(
        SetupRequired(
          SetupStep.totp,
          secret: r.secret,
          otpauthUrl: r.otpauthUrl,
        ),
      );
    } on ApiError catch (err) {
      emit(
        err.code == 'password_change_required'
            ? SetupRequired(SetupStep.password)
            : Unauthenticated(error: err),
      );
    }
  }
}
