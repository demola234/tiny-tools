// Package cli builds the mockmachina command tree and maps its results to exit
// codes. It is the only package that writes to the terminal, and it does so
// only through each command's writers.
package cli
