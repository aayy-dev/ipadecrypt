package macho

import (
	"encoding/binary"
	"testing"
)

func thinWithBuildVersion(minos uint32) []byte {
	cmd := make([]byte, 24)
	binary.LittleEndian.PutUint32(cmd[0:], lcBuildVersion)
	binary.LittleEndian.PutUint32(cmd[4:], 24)
	binary.LittleEndian.PutUint32(cmd[8:], 2) // platform iOS
	binary.LittleEndian.PutUint32(cmd[12:], minos)
	buf := make([]byte, 32+len(cmd))
	binary.LittleEndian.PutUint32(buf[0:], Magic64LE)
	binary.LittleEndian.PutUint32(buf[16:], 1)                // ncmds
	binary.LittleEndian.PutUint32(buf[20:], uint32(len(cmd))) // sizeofcmds
	copy(buf[32:], cmd)
	return buf
}

func TestEncodeOSVersion(t *testing.T) {
	for in, want := range map[string]uint32{"16.5.1": 0x100501, "17.0": 0x110000, "26.1": 0x1a0100, "16.5": 0x100500} {
		if got := EncodeOSVersion(in); got != want {
			t.Fatalf("EncodeOSVersion(%q)=%#x want %#x", in, got, want)
		}
	}
}

func TestSliceMinOS(t *testing.T) {
	v, ok := SliceMinOS(thinWithBuildVersion(0x1a0100)) // 26.1
	if !ok || v != 0x1a0100 {
		t.Fatalf("SliceMinOS=%#x ok=%v, want 0x1a0100", v, ok)
	}
	if v > EncodeOSVersion("16.5.1") != true {
		t.Fatal("26.1 should exceed 16.5.1")
	}
	if _, ok := SliceMinOS([]byte("not macho")); ok {
		t.Fatal("non-Mach-O should report ok=false")
	}
}
