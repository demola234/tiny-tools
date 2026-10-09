import 'dart:convert';

import 'package:http/http.dart' as http;

class User {
  const User({required this.id, required this.name});

  final String id;
  final String name;
}

sealed class UsersResult {
  const UsersResult({this.mockState});

  final String? mockState;
}

final class UsersLoaded extends UsersResult {
  const UsersLoaded(this.users, {super.mockState});

  final List<User> users;
}

final class SessionExpired extends UsersResult {
  const SessionExpired({super.mockState});
}

final class UsersFailed extends UsersResult {
  const UsersFailed({super.mockState});
}

class UsersApi {
  UsersApi(this.baseUrl, {this.mockState, http.Client? client}) : _client = client ?? http.Client();

  final Uri baseUrl;
  final String? mockState;
  final http.Client _client;

  Future<UsersResult> fetchUsers() async {
    try {
      final response = await _client.get(
        baseUrl.resolve('/users'),
        headers: {'X-Mock-State': ?mockState},
      );
      final served = response.headers['x-mock-state'];
      if (response.statusCode == 401) {
        return SessionExpired(mockState: served);
      }
      if (response.statusCode != 200) {
        return UsersFailed(mockState: served);
      }
      final json = jsonDecode(response.body) as Map<String, dynamic>;
      final users = [
        for (final user in json['users'] as List<dynamic>)
          User(id: user['id'] as String, name: user['name'] as String),
      ];
      return UsersLoaded(users, mockState: served);
    } on Exception {
      return const UsersFailed();
    }
  }
}
