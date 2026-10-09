# HTTPS

Plain `http://` works for most development (see [Mobile setup](mobile-setup.md)), but sometimes you need HTTPS. Your app might refuse cleartext, use cookies marked `Secure`, use a library that insists on `https://`, or you might not want debug-only exceptions in your app config at all.

`mockmachina start --https` serves the mock over HTTPS with a certificate your devices can trust:

```text
$ mockmachina start --https
created a local certificate authority in /Users/ada/Library/Application Support/mockmachina/ca; run mockmachina cert --install to trust it
serving 4 routes from .mockmachina on https://127.0.0.1:4001 (Ctrl+C to stop)
```

The port is the same one plain HTTP uses; only the scheme changes. `--https` serves HTTPS only, so switch the base URL in your app to `https://`.

## How it works

The first `--https` creates a **local certificate authority** (CA) on your computer, the same way [mkcert](https://github.com/FiloSottile/mkcert) does. Each `start --https` then makes a fresh certificate signed by that CA, valid for every name your app might use:

- `localhost`, `127.0.0.1` and `::1`;
- `10.0.2.2`, the Android emulator's name for your computer;
- your computer's Wi-Fi and Ethernet addresses, so phones on your network can connect;
- your computer's name, and `<name>.local`;
- the address you give with `--host`.

Trust the CA once on each device and every certificate it signs is trusted from then on, including after your Wi-Fi address changes.

| | |
| --- | --- |
| Where the CA lives | `mockmachina/ca` in your user config folder: `~/Library/Application Support` on macOS, `~/.config` on Linux, `%AppData%` on Windows. Set `MOCKMACHINA_CA_DIR` to use another folder |
| Files | `rootCA.pem` (the certificate, safe to share) and `rootCA-key.pem` (the private key, readable only by you) |
| How long it lasts | The CA lasts 10 years. Each server certificate lasts 397 days, the most browsers accept, and is remade on every start |
| Key type | ECDSA P-256 |

**Keep `rootCA-key.pem` private.** Anyone with it can make certificates that your devices trust for any website. It never leaves your computer, and it doesn't belong in your repository.

To see where the CA is and what to do with it:

```sh
mockmachina cert          # where it is, and the steps for each device
mockmachina cert --pem    # print the certificate, to copy to a device
mockmachina cert --install
```

## Trusting it

| Device | What to do |
| --- | --- |
| Your computer (browsers, curl, desktop apps) | `mockmachina cert --install` |
| iOS Simulator | `mockmachina cert --install` with the simulator booted, or `xcrun simctl keychain booted add-root-cert <path to rootCA.pem>` |
| iPhone or iPad | Install the profile, then turn on full trust ([below](#iphone-and-ipad)) |
| Android emulator or phone | Ship the CA in your debug build ([below](#android)) |
| Flutter on Android | Also trust it in Dart ([below](#flutter)) |

`cert --install` runs these commands and shows each one first:

| System | Commands |
| --- | --- |
| macOS | `security add-trusted-cert -r trustRoot -k ~/Library/Keychains/login.keychain-db rootCA.pem`, then the `xcrun simctl` line above if a simulator is booted. macOS asks for your password |
| Linux | `sudo cp rootCA.pem /usr/local/share/ca-certificates/mockmachina.crt` then `sudo update-ca-certificates` (Debian and Ubuntu) |
| Windows | `certutil -user -addstore Root rootCA.pem` |

Firefox keeps its own list of trusted CAs. To use it there, import `rootCA.pem` under Settings > Privacy & Security > Certificates > View Certificates > Authorities.

### iPhone and iPad

1. Send `rootCA.pem` to the device, by AirDrop or as an email attachment, and open it. iOS says a profile was downloaded.
2. Install it in **Settings > General > VPN & Device Management**.
3. Turn on full trust in **Settings > General > About > Certificate Trust Settings**.

Then start the mock with `--host 0.0.0.0 --https`, and point the app at `https://<your computer's IP>:4001` or `https://<your computer's name>.local:4001`.

### Android

Android apps don't trust certificates that users install unless the app says so. The simplest setup puts the CA in your **debug build only**, so nothing on the device needs changing:

1. Copy `rootCA.pem` to `android/app/src/debug/res/raw/mockmachina_ca.pem`.
2. Add `android/app/src/debug/res/xml/network_security_config.xml`:

   ```xml
   <network-security-config>
       <debug-overrides>
           <trust-anchors>
               <certificates src="@raw/mockmachina_ca"/>
               <certificates src="system"/>
           </trust-anchors>
       </debug-overrides>
   </network-security-config>
   ```

3. Point the debug manifest at it, in `android/app/src/debug/AndroidManifest.xml`:

   ```xml
   <manifest xmlns:android="http://schemas.android.com/apk/res/android">
       <uses-permission android:name="android.permission.INTERNET"/>
       <application android:networkSecurityConfig="@xml/network_security_config"/>
   </manifest>
   ```

Release builds use neither file. The emulator reaches the mock at `https://10.0.2.2:4001`.

`rootCA.pem` is a public certificate, so committing it to your app's repository is fine, but each developer's CA is different. Either have each developer copy their own in (and leave the file out of git), or share one CA by pointing everyone's `MOCKMACHINA_CA_DIR` at the same folder.

### Flutter

On iOS and macOS, Flutter uses the system's trusted CAs, so the steps above are enough.

On Android, Dart's own HTTP client doesn't read the network security config, so trust the CA in Dart as well, in debug builds only:

```dart
import 'dart:io';
import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';

Future<void> trustMockCertificate() async {
  if (!kDebugMode) return;
  final ca = await rootBundle.load('assets/dev/mockmachina_ca.pem');
  SecurityContext.defaultContext.setTrustedCertificatesBytes(ca.buffer.asUint8List());
}
```

Call it in `main()` before the first request, and add `assets/dev/mockmachina_ca.pem` to the assets in `pubspec.yaml`.

## Your own certificate

If your team already has a development certificate, use it instead of the local CA:

```sh
mockmachina start --tls-cert dev.crt --tls-key dev.key
```

Both files are PEM. `--tls-cert` and `--tls-key` go together, and with them MockMachina doesn't touch the local CA.

## In Docker

Inside a container the local CA would be made afresh each time, and nothing would trust it. Make the CA on your computer first, then mount it:

```sh
mockmachina cert
docker run --rm -p 4001:4001 --user "$(id -u):$(id -g)" \
  -v "$PWD/.mockmachina:/mock/.mockmachina" \
  -v "$HOME/Library/Application Support/mockmachina/ca:/ca:ro" -e MOCKMACHINA_CA_DIR=/ca \
  ghcr.io/demola234/mockmachina start --plain --https --host 0.0.0.0 --dir /mock/.mockmachina
```

On Linux the CA is in `~/.config/mockmachina/ca`. `--user` lets the container read your private key, which only you can read.

The certificate covers `localhost`, so apps on the same computer connect to `https://localhost:4001`. It doesn't cover your computer's Wi-Fi address, which the container can't see. For phones on your network, run MockMachina outside Docker, or use `--tls-cert` with a certificate for that address.

## Removing it

1. Untrust the CA. On macOS, delete "MockMachina local CA" in Keychain Access. On Linux, delete `/usr/local/share/ca-certificates/mockmachina.crt` and run `sudo update-ca-certificates --fresh`. On Windows, delete it from `certmgr.msc` under Trusted Root Certification Authorities. On devices, delete the profile or the raw resource.
2. Delete the CA folder that `mockmachina cert` prints.

The next `--https` makes a new CA, which you'd trust again.

## When it doesn't work

| What you see | Why, and the fix |
| --- | --- |
| The browser or app says the certificate isn't trusted | The CA isn't trusted on that device yet. Follow the steps for it above |
| "certificate is not valid for this name" | The app uses an address the certificate doesn't cover, like a new Wi-Fi address. Restart `start --https` so it makes a certificate for your current addresses |
| iPhone: the profile installed but requests still fail | Full trust is off. Turn it on in Certificate Trust Settings |
| Android: `CertPathValidatorException` | The network security config isn't in the build, or the app runs a release build |
| Flutter on Android: `HandshakeException` | Dart doesn't trust the CA. Add the Flutter step above |
| `the local CA in … is incomplete` | One of the two files is missing. Delete the folder and start again; devices need the new CA |
