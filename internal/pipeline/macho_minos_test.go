package pipeline

import (
	"encoding/binary"
	"testing"
)

// thinMachO builds a minimal little-endian 64-bit Mach-O header with a single
// LC_BUILD_VERSION load command carrying the given packed minos.
func thinMachO(minos uint32) []byte {
	const hdr = 32
	cmd := make([]byte, 24) // LC_BUILD_VERSION: cmd,cmdsize,platform,minos,sdk,ntools
	binary.LittleEndian.PutUint32(cmd[0:], lcBuildVersion)
	binary.LittleEndian.PutUint32(cmd[4:], 24)
	binary.LittleEndian.PutUint32(cmd[8:], 2) // platform = iOS
	binary.LittleEndian.PutUint32(cmd[12:], minos)
	binary.LittleEndian.PutUint32(cmd[16:], minos) // sdk
	binary.LittleEndian.PutUint32(cmd[20:], 0)

	buf := make([]byte, hdr+len(cmd))
	binary.LittleEndian.PutUint32(buf[0:], machoMagic64)
	binary.LittleEndian.PutUint32(buf[16:], 1)                // ncmds
	binary.LittleEndian.PutUint32(buf[20:], uint32(len(cmd))) // sizeofcmds
	copy(buf[hdr:], cmd)

	return buf
}

func minosOf(data []byte) uint32 { return binary.LittleEndian.Uint32(data[32+12:]) }

func TestEncodeVersion(t *testing.T) {
	cases := map[string]uint32{"16.5.1": 0x100501, "17.0": 0x110000, "16.5": 0x100500}
	for in, want := range cases {
		if got := encodeVersion(in); got != want {
			t.Fatalf("encodeVersion(%q) = %#x, want %#x", in, got, want)
		}
	}
}

func TestPatchMachOMinOSLowers(t *testing.T) {
	m := thinMachO(0x110000) // 17.0
	if !patchMachOMinOS(m, encodeVersion("16.5.1")) {
		t.Fatal("expected patch to report a change")
	}
	if got := minosOf(m); got != 0x100501 {
		t.Fatalf("minos = %#x, want 0x100501 (16.5.1)", got)
	}
	// The platform field (offset 8 in LC_BUILD_VERSION) must be left intact.
	if plat := binary.LittleEndian.Uint32(m[32+8:]); plat != 2 {
		t.Fatalf("platform = %d, want 2 (iOS); min-OS patch clobbered it", plat)
	}
}

func TestPatchMachOMinOSLeavesLowerAlone(t *testing.T) {
	m := thinMachO(0x0f0000) // 15.0, already runnable
	before := minosOf(m)
	if patchMachOMinOS(m, encodeVersion("16.5.1")) {
		t.Fatal("did not expect a change for an already-low min-OS")
	}
	if minosOf(m) != before {
		t.Fatal("minos should be untouched")
	}
}

func TestPatchMachOMinOSIgnoresNonMachO(t *testing.T) {
	if patchMachOMinOS([]byte("\x89PNG\r\n\x1a\n not a mach-o"), encodeVersion("16.5.1")) {
		t.Fatal("non-Mach-O data should not be reported as changed")
	}
}
