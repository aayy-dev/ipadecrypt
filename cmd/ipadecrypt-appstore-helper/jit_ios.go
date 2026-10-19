//go:build ios

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// enableJIT marks this process as debugged (CS_DEBUGGED) so the bundled ipatool
// SAP emulator (Unicorn) can execute its JIT memory.
//
// On modern jailbroken iOS (Dopamine/roothide, iOS 16+ through 26) JIT is gated
// behind the kernel's CS_DEBUGGED flag ("allowed to run with invalid pages"),
// not an entitlement; without it the emulator's executable mapping is refused.
// jbctl is a jailbreak platform binary permitted to set the flag. Best effort:
// a no-op on other platforms and harmless where jbctl is absent. The bundled
// Unicorn is patched for split W^X (see app/patches/unicorn-ios.patch), which
// relies on this flag being set before the emulator runs.
func enableJIT() {
	pid := strconv.Itoa(os.Getpid())

	for _, jbctl := range []string{"/var/jb/usr/bin/jbctl", "jbctl"} {
		out, err := exec.Command(jbctl, "proc_set_debugged", pid).CombinedOutput()
		if err == nil {
			fmt.Fprintf(os.Stderr, "ipadecrypt: enabled JIT via %s: %s\n", jbctl, strings.TrimSpace(string(out)))

			return
		}
	}
}
