@Timeout(Duration(seconds: 60))
library;

import 'dart:async';
import 'dart:convert';
import 'dart:io';

import 'package:flutter/material.dart';
import 'package:flutter_shop/cart_screen.dart';
import 'package:flutter_shop/shop_api.dart';
import 'package:flutter_test/flutter_test.dart';

late Process mock;
late Uri baseUrl;

Future<Uri> startMock() async {
  final binary = Platform.environment['MOCKMACHINA'] ?? '../../bin/mockmachina';
  mock = await Process.start(binary, ['start', '--port', '0', '--dir', '.mockmachina', '--seed', '7']);
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

  Future<T> real<T>(WidgetTester tester, Future<T> Function(ShopApi api) call) async {
    final result = await tester.runAsync(
      () => HttpOverrides.runWithHttpOverrides(() => call(ShopApi(baseUrl)), _RealNetwork()),
    );
    return result as T;
  }

  testWidgets('a wrong password is refused, the right one signs in', (tester) async {
    expect(await real(tester, (api) => api.signIn('ada@example.com', 'nope')), isA<WrongPassword>());
    final signedIn = await real(tester, (api) => api.signIn('ada@example.com', 'secret'));
    expect(signedIn, isA<SignedIn>());
    expect((signedIn as SignedIn).token, matches(RegExp(r'^[0-9a-f-]{36}$')));
  });

  testWidgets('the cart starts from the data file and remembers additions', (tester) async {
    final before = await real(tester, (api) => api.cart());
    expect(before.map((i) => i.sku), contains('tea'));

    final added = await real(tester, (api) => api.addToCart('bread', 2));
    expect(added.sku, 'bread');
    expect(added.id, isNotEmpty);

    final after = await real(tester, (api) => api.cart());
    expect(after.map((i) => i.id), contains(added.id));
  });

  testWidgets('checkout reports a dropped connection', (tester) async {
    expect(await real(tester, (api) => api.checkout()), CheckoutResult.connectionLost);
  });

  testWidgets('the cart screen lists items with their quantities', (tester) async {
    final items = await real(tester, (api) => api.cart());
    await tester.pumpWidget(MaterialApp(home: CartScreen(load: () async => items)));
    await tester.pump();
    expect(find.text('tea'), findsOneWidget);
    expect(find.text('× 1'), findsOneWidget);
  });

  testWidgets('the cart screen shows when the cart is empty', (tester) async {
    await tester.pumpWidget(MaterialApp(home: CartScreen(load: () async => const <CartItem>[])));
    await tester.pump();
    expect(find.text('Your cart is empty'), findsOneWidget);
  });
}

class _RealNetwork extends HttpOverrides {}
