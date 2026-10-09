# Installing MockMachina

MockMachina is one program, `mockmachina`, with no runtime to install. Pick whichever way suits your machine. Every release is tagged `mock_machina/vX.Y.Z` on [GitHub Releases](https://github.com/demola234/tiny-tools/releases), with archives for macOS, Linux and Windows on Intel and ARM, a `checksums.txt` and an SBOM.

| Where | Command |
| --- | --- |
| macOS or Linux, with Homebrew | `brew install demola234/tap/mockmachina` |
| macOS or Linux, without Homebrew | `curl -fsSL https://raw.githubusercontent.com/demola234/tiny-tools/main/mock_machina/install.sh \| sh` |
| Windows, with Scoop | `scoop bucket add demola234 https://github.com/demola234/scoop-bucket` then `scoop install mockmachina` |
| Anywhere with Go 1.25 or newer | `go install github.com/demola234/tiny-tools/mock_machina/cmd/mockmachina@latest` |
| Docker | `docker run --rm -p 4001:4001 -v "$PWD/.mockmachina:/mock/.mockmachina" ghcr.io/demola234/mockmachina` |

Then check it works:

```sh
mockmachina --version
```

## The install script

`install.sh` finds the newest MockMachina release (the repository also releases other tools, so it looks for `mock_machina/v*` tags), downloads the archive for your system, checks it against `checksums.txt`, and puts `mockmachina` in `/usr/local/bin`, or `~/.local/bin` if it can't write there. It refuses to install an archive whose checksum doesn't match.

Settings, as environment variables before `sh`:

| Variable | What it does |
| --- | --- |
| `MOCKMACHINA_VERSION` | Install this version instead of the newest, like `v0.7.0` |
| `MOCKMACHINA_INSTALL_DIR` | Install into this folder |

```sh
curl -fsSL https://raw.githubusercontent.com/demola234/tiny-tools/main/mock_machina/install.sh | MOCKMACHINA_VERSION=v0.7.0 sh
```

To install by hand, download the archive for your system from the release, check its line in `checksums.txt` with `shasum -a 256`, and copy `mockmachina` (or `mockmachina.exe`) onto your `PATH`.

## Docker

The image `ghcr.io/demola234/mockmachina` runs on amd64 and arm64. It holds only the `mockmachina` binary, runs as a non-root user, and by default serves `/mock/.mockmachina` on port 4001 to anything that can reach the container.

```sh
docker run --rm -p 4001:4001 -v "$PWD/.mockmachina:/mock/.mockmachina" ghcr.io/demola234/mockmachina
```

- **Colima and other VM-based runtimes** only share your home folder with the container by default. A project elsewhere, like under `/tmp` or another disk, mounts as an empty folder and serves 0 routes; move it under your home folder or add the path to Colima's `mounts`.
- **Edits apply live.** The image checks the folder for changes every 500 ms, because file change events don't cross into containers from bind mounts.
- **State switches write to your files.** `state set` and the live screen change `routes/*.yaml`, so mount the folder read-write if you use them, and read-only (`:ro`) if you don't.
- **Pick a version** with a tag, like `ghcr.io/demola234/mockmachina:v0.7.0`. `latest` follows the newest release.
- **Other commands** work by naming them: `docker run --rm -v "$PWD/.mockmachina:/mock/.mockmachina" ghcr.io/demola234/mockmachina lint --dir /mock/.mockmachina`.
- **Seeds and proxies** are flags as usual: add `start --plain --host 0.0.0.0 --dir /mock/.mockmachina --seed 42` after the image name to replace the default command.

In Docker Compose, next to the app that uses it:

```yaml
services:
  mock:
    image: ghcr.io/demola234/mockmachina:latest
    ports: ["4001:4001"]
    volumes: ["./.mockmachina:/mock/.mockmachina"]
```

Other containers in the same Compose project reach it at `http://mock:4001`.

For HTTPS in a container, see [HTTPS](https.md#in-docker).

## Updating and removing

`mockmachina update` finds the latest release and updates whichever way it was installed: it runs `brew upgrade`, `scoop update` or `go install` for you, and replaces itself in place (after checking the checksum) when it came from the install script or a download. `mockmachina update --check` only tells you whether a newer release is out.

When you run mockmachina in a terminal and a newer release is out, it asks first:

```text
↑ mockmachina v0.2.0 is out (you have v0.1.1). Update now? [Y/n]
⠴ downloading mockmachina v0.2.0 ▰▰▰▰▰▰▰▰▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱▱ 2.2 / 6.2 MB
```

Enter updates, then your command carries on (the new version is used from the next run). `n` skips that version for good. It checks GitHub at most once a day, remembers the answer in your user cache folder, and stays quiet in CI, in pipes and scripts, and when GitHub can't be reached. Set `MOCKMACHINA_NO_UPDATE_CHECK=1` to turn it off.

To do it by hand:

| Installed with | Update | Remove |
| --- | --- | --- |
| Homebrew | `brew upgrade mockmachina` | `brew uninstall mockmachina` |
| Scoop | `scoop update mockmachina` | `scoop uninstall mockmachina` |
| The install script | Run it again | Delete the `mockmachina` file it printed |
| Go | Run `go install …@latest` again | Delete `$(go env GOPATH)/bin/mockmachina` |

Removing the program leaves your projects alone. If you used HTTPS, the local certificate authority stays in your user config folder until you delete it; see [HTTPS](https.md#removing-it).
