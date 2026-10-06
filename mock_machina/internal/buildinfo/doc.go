// Package buildinfo reports which build of mockmachina is running: its version,
// commit and whether it was built from a modified tree. It reads what the Go
// toolchain embeds in every binary, so it needs no build flags; release builds
// may still set the version with -ldflags.
package buildinfo
