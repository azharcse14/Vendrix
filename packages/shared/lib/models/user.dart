import 'json.dart';

/// Customer er profile (`GET /api/me`, verify er `user`; 7.3). Id nai (client er lage na).
class User {
  /// Phone (8.1 er normalize kora).
  final String phone;

  /// Naam, email (na thakle null).
  final String? name, email;

  /// SMS er bhasha (`bn`, `en`).
  final String locale;

  /// User.
  const User(this.phone, this.name, this.email, this.locale);

  /// JSON theke.
  factory User.fromJson(Map<String, dynamic> j) => User(
    field(j, 'phone'),
    field(j, 'name'),
    field(j, 'email'),
    field(j, 'locale'),
  );
}
