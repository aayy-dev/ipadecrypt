//go:build !ios

package main

// enableJIT is only needed on iOS, where the bundled ipatool SAP emulator runs
// JIT code under a jailbreak's code-signing enforcement.
func enableJIT() {}
