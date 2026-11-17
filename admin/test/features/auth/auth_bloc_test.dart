import 'dart:convert';

import 'package:admin/core/auth/auth_bloc.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:shared/shared.dart';

const me = {
  'name': 'Rahim',
  'email': 'owner@example.com',
  'role': 'owner',
  'vendor_id': null,
};

http.Response json(int status, Object? body) => http.Response(
  body == null ? '' : jsonEncode(body),
  status,
  headers: {'content-type': 'application/json'},
);

http.Response err(int status, String code, [Map<String, String>? fields]) =>
    json(status, {
      'error': {'code': code, 'message': code, 'fields': ?fields},
    });

/// Fake API: "METHOD path" → response er queue (protiti call e ekta); [calls] e kram ar body.
class FakeApi {
  final Map<String, List<http.Response>> routes;
  final calls = <String>[];
  final bodies = <String, Object?>{};
  FakeApi(this.routes);

  AuthBloc bloc() => AuthBloc(
    AdminAuthRepository(
      ApiClient(
        Uri.parse('https://api.test'),
        MockClient((req) async {
          final key = '${req.method} ${req.url.path}';
          calls.add(key);
          if (req.body.isNotEmpty) bodies[key] = jsonDecode(req.body);
          final q = routes[key];
          if (q == null || q.isEmpty) return err(404, 'not_found');
          return q.length == 1 ? q.first : q.removeAt(0);
        }),
      ),
    ),
  );
}

void main() {
  test('shuru: session nai → Unauthenticated (error chhara)', () async {
    final api = FakeApi({
      'GET /api/admin/me': [err(401, 'unauthenticated')],
    });
    final b = api.bloc()..add(AuthStarted());
    final s = await b.stream.first;
    expect(s, isA<Unauthenticated>());
    expect((s as Unauthenticated).error, isNull);
  });

  test('shuru: network bhul o Unauthenticated (app atke na)', () async {
    final b = AuthBloc(
      AdminAuthRepository(
        ApiClient(
          Uri.parse('https://api.test'),
          MockClient((_) async => throw http.ClientException('x')),
        ),
      ),
    )..add(AuthStarted());
    final s = await b.stream.first;
    expect((s as Unauthenticated).error?.code, 'network_error');
  });

  test('shuru: active session → Authenticated', () async {
    final api = FakeApi({
      'GET /api/admin/me': [json(200, me)],
    });
    final b = api.bloc()..add(AuthStarted());
    final s = await b.stream.first;
    expect((s as Authenticated).staff.role, 'owner');
  });

  test('login → TOTP → dashboard (8.3)', () async {
    final api = FakeApi({
      'POST /api/admin/auth/login': [
        json(200, {'totp_required': true, 'temp_token': 'tt'}),
      ],
      'POST /api/admin/auth/totp': [
        err(400, 'mfa_invalid'),
        json(200, {'setup_required': false}),
      ],
      'GET /api/admin/me': [json(200, me)],
    });
    final b = api.bloc();
    b.add(LoginSubmitted('sobujbazar', 'owner@example.com', 'pw'));
    expect(await b.stream.first, isA<TotpRequired>());
    expect(api.bodies['POST /api/admin/auth/login'], {
      'shop': 'sobujbazar',
      'email': 'owner@example.com',
      'password': 'pw',
    });

    b.add(MfaSubmitted('000000'));
    final bad = await b.stream.first as TotpRequired;
    expect(bad.error?.code, 'mfa_invalid');
    expect(bad.tempToken, 'tt');

    b.add(MfaSubmitted('123456'));
    expect(await b.stream.first, isA<Authenticated>());
    expect(api.bodies['POST /api/admin/auth/totp'], {
      'temp_token': 'tt',
      'code': '123456',
    });
  });

  test('recovery code diye login', () async {
    final api = FakeApi({
      'POST /api/admin/auth/login': [
        json(200, {'totp_required': true, 'temp_token': 'tt'}),
      ],
      'POST /api/admin/auth/recovery': [
        json(200, {'setup_required': false}),
      ],
      'GET /api/admin/me': [json(200, me)],
    });
    final b = api.bloc()..add(LoginSubmitted('s', 'e@x.com', 'pw'));
    await b.stream.first;
    b.add(MfaSubmitted('ABCDE-FGHIJ', recovery: true));
    expect(await b.stream.first, isA<Authenticated>());
    expect(api.calls, contains('POST /api/admin/auth/recovery'));
    expect(api.calls, isNot(contains('POST /api/admin/auth/totp')));
  });

  test('bhul password: Unauthenticated + error', () async {
    final api = FakeApi({
      'POST /api/admin/auth/login': [err(401, 'invalid_credentials')],
    });
    final b = api.bloc()..add(LoginSubmitted('s', 'e@x.com', 'bad'));
    final s = await b.stream.first as Unauthenticated;
    expect(s.error?.code, 'invalid_credentials');
  });

  test('TOTP er temp_token expire → login e ferot', () async {
    final api = FakeApi({
      'POST /api/admin/auth/login': [
        json(200, {'totp_required': true, 'temp_token': 'tt'}),
      ],
      'POST /api/admin/auth/totp': [err(401, 'session_expired')],
    });
    final b = api.bloc()..add(LoginSubmitted('s', 'e@x.com', 'pw'));
    await b.stream.first;
    b.add(MfaSubmitted('123456'));
    final s = await b.stream.first as Unauthenticated;
    expect(s.error?.code, 'session_expired');
  });

  test('prothom login: setup_pending → password → TOTP setup/enable → recovery code → dashboard', () async {
    final api = FakeApi({
      'POST /api/admin/auth/login': [
        json(200, {'setup_required': true}),
      ],
      'POST /api/admin/auth/totp/setup': [
        err(403, 'password_change_required'),
        json(200, {
          'otpauth_url': 'otpauth://totp/s:e?secret=ABC&issuer=s',
          'secret': 'ABC',
        }),
      ],
      'POST /api/admin/auth/password': [
        err(400, 'validation_failed', {'old': 'wrong'}),
        json(204, null),
      ],
      'GET /api/admin/me': [err(403, 'setup_required'), json(200, me)],
      'POST /api/admin/auth/totp/enable': [
        err(400, 'mfa_invalid'),
        json(200, {
          'recovery_codes': ['AAAAA-BBBBB', 'CCCCC-DDDDD'],
        }),
      ],
    });
    final b = api.bloc()..add(LoginSubmitted('s', 'e@x.com', 'temp'));

    var s = await b.stream.first as SetupRequired;
    expect(s.step, SetupStep.password);

    b.add(PasswordSubmitted('bad', 'new-password-123'));
    s = await b.stream.first as SetupRequired;
    expect(s.step, SetupStep.password);
    expect(s.error?.fields, {'old': 'wrong'});

    b.add(PasswordSubmitted('temp', 'new-password-123'));
    s = await b.stream.first as SetupRequired;
    expect(s.step, SetupStep.totp);
    expect(s.secret, 'ABC');
    expect(s.otpauthUrl, startsWith('otpauth://'));
    expect(api.bodies['POST /api/admin/auth/password'], {
      'old': 'temp',
      'new': 'new-password-123',
    });

    b.add(TotpEnableSubmitted('000000'));
    s = await b.stream.first as SetupRequired;
    expect(s.step, SetupStep.totp);
    expect(s.secret, 'ABC'); // bhul code e notun secret na
    expect(s.error?.code, 'mfa_invalid');

    b.add(TotpEnableSubmitted('123456'));
    s = await b.stream.first as SetupRequired;
    expect(s.step, SetupStep.recoveryCodes);
    expect(s.recoveryCodes, ['AAAAA-BBBBB', 'CCCCC-DDDDD']);
    expect(s.toString(), isNot(contains('AAAAA'))); // log e secret na

    b.add(RecoveryCodesSaved());
    expect(await b.stream.first, isA<Authenticated>());
    // setup e totp/setup duibar: shuru te (password lage) ar password er por
    expect(
      api.calls.where((c) => c == 'POST /api/admin/auth/totp/setup').length,
      2,
    );
  });

  test(
    'TOTP live + must_change_password: password er por shoja dashboard',
    () async {
      final api = FakeApi({
        'POST /api/admin/auth/login': [
          json(200, {'totp_required': true, 'temp_token': 'tt'}),
        ],
        'POST /api/admin/auth/totp': [
          json(200, {'setup_required': true}),
        ],
        'POST /api/admin/auth/totp/setup': [
          err(403, 'password_change_required'),
        ],
        'POST /api/admin/auth/password': [json(204, null)],
        'GET /api/admin/me': [json(200, me)],
      });
      final b = api.bloc()..add(LoginSubmitted('s', 'e@x.com', 'pw'));
      await b.stream.first;
      b.add(MfaSubmitted('123456'));
      expect((await b.stream.first as SetupRequired).step, SetupStep.password);
      b.add(PasswordSubmitted('pw', 'new-password-123'));
      expect(await b.stream.first, isA<Authenticated>());
    },
  );

  test('reset er por (password bodol lage na): shoja TOTP setup', () async {
    final api = FakeApi({
      'GET /api/admin/me': [err(403, 'setup_required')],
      'POST /api/admin/auth/totp/setup': [
        json(200, {'otpauth_url': 'otpauth://x', 'secret': 'S'}),
      ],
    });
    final b = api.bloc()..add(AuthStarted());
    final s = await b.stream.first as SetupRequired;
    expect(s.step, SetupStep.totp);
  });

  test('logout: API fail holeo Unauthenticated', () async {
    final api = FakeApi({
      'GET /api/admin/me': [json(200, me)],
      'POST /api/admin/auth/logout': [err(500, 'internal')],
    });
    final b = api.bloc()..add(AuthStarted());
    await b.stream.first;
    b.add(LogoutRequested());
    final s = await b.stream.first as Unauthenticated;
    expect(s.error, isNull);
    expect(api.calls.last, 'POST /api/admin/auth/logout');
  });
}
