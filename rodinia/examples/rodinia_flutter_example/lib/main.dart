import 'dart:async';

import 'package:flutter/material.dart';
import 'package:path_provider/path_provider.dart';
import 'package:rodinia_flutter/rodinia_store.dart';

Future<void> main() async {
  WidgetsFlutterBinding.ensureInitialized();
  final dir = await getApplicationDocumentsDirectory();
  final store = await RodiniaStore.open(dir.path);
  runApp(RodiniaStoreExample(store: store));
}

class RodiniaStoreExample extends StatefulWidget {
  const RodiniaStoreExample({super.key, required this.store});

  final RodiniaStore store;

  @override
  State<RodiniaStoreExample> createState() => _RodiniaStoreExampleState();
}

class _RodiniaStoreExampleState extends State<RodiniaStoreExample> {
  final _events = <String>[];
  late final StreamSubscription<StorageEvent> _subscription;
  late final TextEditingController _themeController;
  late final TextEditingController _lookupKeyController;
  late final TextEditingController _lookupValueController;
  bool _lookupEncrypted = false;
  String? _lookupResult;

  RodiniaStore get _store => widget.store;

  @override
  void initState() {
    super.initState();
    _themeController = TextEditingController(
      text: _store.get<String>('theme') ?? 'light',
    );
    _lookupKeyController = TextEditingController();
    _lookupValueController = TextEditingController();
    _subscription = _store.watch().listen((event) {
      setState(() => _events.insert(0, event.toString()));
    });
  }

  @override
  void dispose() {
    _subscription.cancel();
    _themeController.dispose();
    _lookupKeyController.dispose();
    _lookupValueController.dispose();
    super.dispose();
  }

  void _saveTheme() => setState(() {
    _store.set('theme', _themeController.text);
  });

  void _incrementVisits() => setState(() {
    _store.increment('visits');
  });

  void _setExpiringFlag() => setState(() {
    _store.set('flash_sale', true, ttl: const Duration(seconds: 5));
  });

  void _setEncryptedSecret() => setState(() {
    _store.setEncryptionKey(RodiniaStore.generateEncryptionKey());
    _store.set('refresh_token', 'super-secret-token', encrypted: true);
  });

  void _lookupKey() {
    final key = _lookupKeyController.text.trim();
    setState(() => _lookupResult = key.isEmpty ? null : _readAsText(key));
  }

void _setLookupValue() {
    final key = _lookupKeyController.text.trim();
    if (key.isEmpty) return;
    try {
      _store.set(key, _lookupValueController.text, encrypted: _lookupEncrypted);
      setState(() => _lookupResult = _readAsText(key));
    } on StoreError catch (e) {
      setState(() {
        _lookupResult = switch (e) {
          StoreError_EncryptionKeyNotSet() =>
            '(no encryption key yet — tap "store.set(..., encrypted: '
                'true)" below once, or uncheck "encrypted")',
          _ => '(error: $e)',
        };
      });
    }
  }

  /// Reads [key] as its raw text representation, whatever type it was
  /// originally `set()` with — `set(k, "hi")` stores UTF-8 text directly,
  /// and `set(k, 5)`/`set(k, true)`/etc. store their JSON text ("5",
  /// "true"), so decoding as plain text is a reasonable generic viewer for
  /// any key without knowing its type up front.
  ///
  /// The encryption key lives only in memory (by design — see
  /// `RodiniaStore.setEncryptionKey`), so if the app is relaunched as a
  /// fresh process after an encrypted value was written in an earlier run,
  /// the on-disk entry is still there but undecryptable until the key is
  /// set again. `store.get()` surfaces that as
  /// `StoreError.encryptionKeyNotSet` rather than silently returning null,
  /// so callers can tell "no value" apart from "value present but its key
  /// isn't loaded".
  String _readAsText(String key) {
    try {
      return _store.get<String>(key) ?? '(not set)';
    } on StoreError catch (e) {
      return switch (e) {
        StoreError_EncryptionKeyNotSet() => '(key not loaded this session)',
        _ => '(error: $e)',
      };
    } on FormatException {
      return '(binary value, not valid UTF-8 text)';
    }
  }

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      home: Scaffold(
        appBar: AppBar(title: const Text('Rodinia-Store')),
        body: ListView(
          padding: const EdgeInsets.all(16),
          children: [
            Text('${_store.length} key(s): ${_store.keys.join(', ')}'),
            const Divider(height: 32),
            TextField(
              controller: _themeController,
              decoration: const InputDecoration(labelText: 'theme'),
            ),
            const SizedBox(height: 8),
            ElevatedButton(
              onPressed: _saveTheme,
              child: const Text('store.set("theme", ...)'),
            ),
            const Divider(height: 32),
            Text('visits = ${_store.get<int>('visits') ?? 0}'),
            ElevatedButton(
              onPressed: _incrementVisits,
              child: const Text('store.increment("visits")'),
            ),
            const Divider(height: 32),
            Text(
              'flash_sale = ${_store.get<bool>('flash_sale') ?? false} '
              '(expires 5s after being set)',
            ),
            ElevatedButton(
              onPressed: _setExpiringFlag,
              child: const Text('store.set("flash_sale", true, ttl: 5s)'),
            ),
            const Divider(height: 32),
            Text(
              'refresh_token = ${_readAsText('refresh_token')} (encrypted on disk)',
            ),
            ElevatedButton(
              onPressed: _setEncryptedSecret,
              child: const Text('store.set(..., encrypted: true)'),
            ),
            const Divider(height: 32),
            const Text(
              'Set / get any key:',
              style: TextStyle(fontWeight: FontWeight.bold),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _lookupKeyController,
              decoration: const InputDecoration(labelText: 'key'),
              onSubmitted: (_) => _lookupKey(),
            ),
            const SizedBox(height: 8),
            TextField(
              controller: _lookupValueController,
              decoration: const InputDecoration(labelText: 'value (for Set)'),
              onSubmitted: (_) => _setLookupValue(),
            ),
            CheckboxListTile(
              contentPadding: EdgeInsets.zero,
              controlAffinity: ListTileControlAffinity.leading,
              title: const Text('encrypted'),
              value: _lookupEncrypted,
              onChanged:
                  (value) => setState(() => _lookupEncrypted = value ?? false),
            ),
            Row(
              children: [
                Expanded(
                  child: ElevatedButton(
                    onPressed: _setLookupValue,
                    child: const Text('store.set(key, value, encrypted:)'),
                  ),
                ),
                const SizedBox(width: 8),
                Expanded(
                  child: ElevatedButton(
                    onPressed: _lookupKey,
                    child: const Text('store.get(key)'),
                  ),
                ),
              ],
            ),
            if (_lookupResult != null) ...[
              const SizedBox(height: 8),
              Text(_lookupResult!),
            ],
            const Divider(height: 32),
            const Text(
              'Live events from store.watch():',
              style: TextStyle(fontWeight: FontWeight.bold),
            ),
            for (final event in _events) Text(event),
          ],
        ),
      ),
    );
  }
}
