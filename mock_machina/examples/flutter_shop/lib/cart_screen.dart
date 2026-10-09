import 'package:flutter/material.dart';

import 'shop_api.dart';

class CartScreen extends StatelessWidget {
  const CartScreen({super.key, required this.load});

  final Future<List<CartItem>> Function() load;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(title: const Text('Cart')),
      body: FutureBuilder<List<CartItem>>(
        future: load(),
        builder: (context, snapshot) {
          final items = snapshot.data;
          return Center(
            child: switch (items) {
              null => const CircularProgressIndicator(),
              [] => const Text('Your cart is empty'),
              _ => ListView(
                  children: [for (final item in items) ListTile(title: Text(item.sku), trailing: Text('× ${item.qty}'))],
                ),
            },
          );
        },
      ),
    );
  }
}
