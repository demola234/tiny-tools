@Timeout(Duration(seconds: 60))
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_shop/users_api.dart';
import 'package:flutter_shop/users_screen.dart';
import 'package:flutter_test/flutter_test.dart';

late Process mock;
late Uri baseUrl;

Future<Uri> startMock() async {
  final binary = Platform.environment['MOCKMACHINA'] ?? '../../bin/mockmachina';
  mock = await Process.start(binary, ['start', '--port', '0', '--dir', '.mockmachina']);
  final serving = Completer<String>();
  mock.stdout.transform(utf8.decoder).transform(const LineSplitter()).listen((line) {
    if (!serving.isCompleted) {
      serving.complete(line);
    }
  });
  final firstLine = await serving.future;
  return Uri.parse(RegExp(r'http://\S+').firstMatch(firstLine)!.group(0)!);
}

void main() {
  TestWidgetsFlutterBinding.ensureInitialized();

  setUpAll(() async => baseUrl = await startMock());
  tearDownAll(() => mock.kill(ProcessSignal.sigint));

  Future<UsersResult> fetch(WidgetTester tester, String? state) async {
    final result = await tester.runAsync(
      () => HttpOverrides.runWithHttpOverrides(
        () => UsersApi(baseUrl, mockState: state).fetchUsers(),
        _RealNetwork(),
      ),
    );
    return result!;
  }

  Future<void> showState(WidgetTester tester, String? state) async {
    final result = await fetch(tester, state);
    await tester.pumpWidget(MaterialApp(home: UsersScreen(load: () async => result)));
    await tester.pump();
  }

  testWidgets('shows a spinner while loading', (tester) async {
    final never = Completer<UsersResult>().future;
    await tester.pumpWidget(MaterialApp(home: UsersScreen(load: () => never)));
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
  });

  testWidgets('lists users from the active state', (tester) async {
    await showState(tester, null);
    expect(find.text('Ada Obi'), findsOneWidget);
    expect(find.text('Tunde Bello'), findsOneWidget);
    expect(find.text('mock: success'), findsOneWidget);
  });

  testWidgets('shows the empty state', (tester) async {
    await showState(tester, 'empty');
    expect(find.text('No users yet'), findsOneWidget);
    expect(find.text('mock: empty'), findsOneWidget);
  });

  testWidgets('shows session expired for a 401', (tester) async {
    await showState(tester, 'unauthorized');
    expect(find.text('Your session expired'), findsOneWidget);
    expect(find.text('Sign in again'), findsOneWidget);
  });

  testWidgets('shows an error with retry for a 500', (tester) async {
    await showState(tester, 'server_error');
    expect(find.text("Couldn't load users"), findsOneWidget);
    expect(find.text('Retry'), findsOneWidget);
  });

  testWidgets('retry shows a spinner, then the new result', (tester) async {
    final failed = await fetch(tester, 'server_error');
    final loaded = await fetch(tester, 'success');
    final retry = Completer<UsersResult>();
    var calls = 0;
    await tester.pumpWidget(MaterialApp(
      home: UsersScreen(load: () => calls++ == 0 ? Future.value(failed) : retry.future),
    ));
    await tester.pump();
    expect(find.text('Retry'), findsOneWidget);

    await tester.tap(find.text('Retry'));
    await tester.pump();
    expect(find.byType(CircularProgressIndicator), findsOneWidget);
    expect(find.text('Retry'), findsNothing);

    retry.complete(loaded);
    await tester.pump();
    expect(find.text('Ada Obi'), findsOneWidget);
    expect(calls, 2);
  });
}

class _RealNetwork extends HttpOverrides {}
