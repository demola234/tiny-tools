import 'dart:convert';

import 'package:http/http.dart' as http;

class CartItem {
  const CartItem({required this.id, required this.sku, required this.qty});

  factory CartItem.fromJson(Map<String, dynamic> json) =>
      CartItem(id: json['id'] as String, sku: json['sku'] as String, qty: json['qty'] as int);

  final String id;
  final String sku;
  final int qty;
}

sealed class SignInResult {
  const SignInResult();
}

final class SignedIn extends SignInResult {
  const SignedIn(this.token);

  final String token;
}

final class WrongPassword extends SignInResult {
  const WrongPassword();
}

final class SignInFailed extends SignInResult {
  const SignInFailed();
}

enum CheckoutResult { placed, rejected, connectionLost }

class ShopApi {
  ShopApi(this.baseUrl, {http.Client? client}) : _client = client ?? http.Client();

  final Uri baseUrl;
  final http.Client _client;

  static const _json = {'Content-Type': 'application/json'};

  Future<SignInResult> signIn(String email, String password) async {
    try {
      final response = await _client.post(
        baseUrl.resolve('/session'),
        headers: _json,
        body: jsonEncode({'email': email, 'password': password}),
      );
      return switch (response.statusCode) {
        201 => SignedIn((jsonDecode(response.body) as Map<String, dynamic>)['token'] as String),
        401 => const WrongPassword(),
        _ => const SignInFailed(),
      };
    } on Exception {
      return const SignInFailed();
    }
  }

  Future<List<CartItem>> cart() async {
    final response = await _client.get(baseUrl.resolve('/cart/items'));
    return [
      for (final item in jsonDecode(response.body) as List<dynamic>) CartItem.fromJson(item as Map<String, dynamic>),
    ];
  }

  Future<CartItem> addToCart(String sku, int qty) async {
    final response = await _client.post(
      baseUrl.resolve('/cart/items'),
      headers: _json,
      body: jsonEncode({'sku': sku, 'qty': qty}),
    );
    return CartItem.fromJson(jsonDecode(response.body) as Map<String, dynamic>);
  }

  Future<CheckoutResult> checkout() async {
    try {
      final response = await _client.post(baseUrl.resolve('/checkout'));
      return response.statusCode == 201 ? CheckoutResult.placed : CheckoutResult.rejected;
    } on http.ClientException {
      return CheckoutResult.connectionLost;
    }
  }
}
