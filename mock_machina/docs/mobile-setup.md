# Mobile setup

`mockmachina start` listens on `127.0.0.1:4001` by default, which only programs on your computer can reach. This page says which address your app should use on each device, and the one-time settings each platform needs.

Pass the address to your Flutter app at build time, rather than hard-coding it:

```sh
flutter run --dart-define=API_BASE_URL=http://10.0.2.2:4001
```

```dart
const apiBaseUrl = String.fromEnvironment('API_BASE_URL', defaultValue: 'http://localhost:4001');
```

## Where the app runs

| Device | Base URL | Extra setup |
| --- | --- | --- |
| iOS Simulator | `http://localhost:4001` | Allow local networking ([below](#ios)) |
| Android Emulator | `http://10.0.2.2:4001` | Allow plain HTTP in debug builds ([below](#android)). `10.0.2.2` is the emulator's name for your computer |
| Android phone over USB | `http://localhost:4001` | Run `adb reverse tcp:4001 tcp:4001` once per connection, plus plain HTTP ([below](#android)) |
| Phone on the same Wi-Fi | `http://<your computer's IP>:4001` | Start with `mockmachina start --host 0.0.0.0`, plus the platform setting below |
| Flutter web or a browser | `http://localhost:4001` | Nothing. CORS is on by default |
| macOS desktop app | `http://localhost:4001` | Add the network client entitlement (`com.apple.security.network.client`) |

To find your computer's IP on a Mac, run `ipconfig getifaddr en0`. On Windows, use `ipconfig`; on Linux, `hostname -I`.

`--host 0.0.0.0` makes the mock reachable by anything on your network, so use it on trusted networks only.

## Android

Android blocks plain `http://` by default. Allow it in **debug builds only** by adding this to `android/app/src/debug/AndroidManifest.xml`:

```xml
<manifest xmlns:android="http://schemas.android.com/apk/res/android">
    <uses-permission android:name="android.permission.INTERNET"/>
    <application android:usesCleartextTraffic="true"/>
</manifest>
```

Release builds keep the default, so this can't leak into production.

## iOS

App Transport Security blocks plain HTTP unless local networking is allowed. Add this inside the top-level `<dict>` of `ios/Runner/Info.plist`:

```xml
<key>NSAppTransportSecurity</key>
<dict>
    <key>NSAllowsLocalNetworking</key>
    <true/>
</dict>
```

`NSAllowsLocalNetworking` only covers local addresses (`localhost`, `.local` names and private IPs), not the public internet.

## Flutter web

Nothing to configure. MockMachina answers browser preflight checks and echoes your page's origin, so `fetch` and `package:http` calls from `flutter run -d chrome` work, including ones with cookies or an `Authorization` header. Use `mockmachina start --no-cors` only if you need to test what happens without CORS.

## Over HTTPS instead

The Android and iOS settings above allow plain HTTP in debug builds. If you'd rather not have them, or your app needs `https://`, serve the mock over HTTPS with a certificate your devices trust:

```sh
mockmachina start --https --host 0.0.0.0
mockmachina cert --install
```

| Device | Base URL |
| --- | --- |
| iOS Simulator | `https://localhost:4001` |
| Android Emulator | `https://10.0.2.2:4001` |
| Phone on the same Wi-Fi | `https://<your computer's IP>:4001` |

The first command creates a local certificate authority on your computer; the second trusts it on your computer and in a booted iOS Simulator. Phones and Android need one more step each, and Flutter on Android needs a line of Dart. [HTTPS](https.md) has all of them.

With HTTPS you can remove `usesCleartextTraffic` and `NSAllowsLocalNetworking`.

## Checking it works

Open `<base URL>/health` in the device's browser. If you ran `mockmachina init`, you'll see `{"status":"up"}`, and the request appears in the terminal running `mockmachina start`. If it doesn't appear, the device isn't reaching your computer: check the address, the platform setting, and (for Wi-Fi) `--host 0.0.0.0`.
