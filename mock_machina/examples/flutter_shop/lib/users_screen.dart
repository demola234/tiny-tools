import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';

import 'users_api.dart';

class UsersScreen extends StatefulWidget {
  const UsersScreen({super.key, required this.load});

  final Future<UsersResult> Function() load;

  @override
  State<UsersScreen> createState() => _UsersScreenState();
}

class _UsersScreenState extends State<UsersScreen> {
  late Future<UsersResult> _result = widget.load();

  void _reload() {
    setState(() {
      _result = widget.load();
    });
  }

  @override
  Widget build(BuildContext context) {
    return FutureBuilder<UsersResult>(
      future: _result,
      builder: (context, snapshot) {
        final result = snapshot.connectionState == ConnectionState.done ? snapshot.data : null;
        return Scaffold(
          appBar: AppBar(
            title: const Text('Users'),
            actions: [
              if (kDebugMode && result?.mockState != null)
                Padding(
                  padding: const EdgeInsets.only(right: 12),
                  child: Chip(label: Text('mock: ${result!.mockState}')),
                ),
            ],
          ),
          body: Center(child: _body(result)),
        );
      },
    );
  }

  Widget _body(UsersResult? result) {
    return switch (result) {
      null => const CircularProgressIndicator(),
      UsersLoaded(users: []) => const Text('No users yet'),
      UsersLoaded(:final users) => ListView(
          children: [for (final user in users) ListTile(title: Text(user.name), subtitle: Text(user.id))],
        ),
      SessionExpired() => _message('Your session expired', 'Sign in again'),
      UsersFailed() => _message("Couldn't load users", 'Retry'),
    };
  }

  Widget _message(String text, String action) {
    return Column(
      mainAxisSize: MainAxisSize.min,
      children: [
        Text(text),
        const SizedBox(height: 12),
        FilledButton(onPressed: _reload, child: Text(action)),
      ],
    );
  }
}
