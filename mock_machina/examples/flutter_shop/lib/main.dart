import 'package:flutter/material.dart';

import 'users_api.dart';
import 'users_screen.dart';

const apiBaseUrl = String.fromEnvironment('API_BASE_URL', defaultValue: 'http://localhost:4001');

void main() {
  final api = UsersApi(Uri.parse(apiBaseUrl));
  runApp(MaterialApp(title: 'Flutter Shop', home: UsersScreen(load: api.fetchUsers)));
}
