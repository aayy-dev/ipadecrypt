package appstore

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"testing"

	ipatool "github.com/majd/ipatool/v2/pkg/appstore"
	"howett.net/plist"
)

func writeIPA(t *testing.T, files map[string]any) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "app.ipa")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}

	zw := zip.NewWriter(f)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}

		data, ok := content.([]byte)
		if !ok {
			if data, err = plist.Marshal(content, plist.BinaryFormat); err != nil {
				t.Fatal(err)
			}
		}

		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}

	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	return path
}

func TestIPAVersion(t *testing.T) {
	path := writeIPA(t, map[string]any{
		"Payload/Claude.app/Info.plist":                        map[string]any{"CFBundleShortVersionString": "1.251015.0", "CFBundleVersion": "1"},
		"Payload/Claude.app/PlugIns/Widget.appex/Info.plist":   map[string]any{"CFBundleShortVersionString": "9.9"},
		"Payload/Claude.app/Frameworks/X.framework/Info.plist": map[string]any{"CFBundleShortVersionString": "8.8"},
		"iTunesMetadata.plist":                                 []byte("<plist/>"),
	})

	got, err := ipaVersion(path)
	if err != nil {
		t.Fatalf("ipaVersion: %v", err)
	}
	if got != "1.251015.0" {
		t.Fatalf("ipaVersion = %q, want the app's own version 1.251015.0", got)
	}
}

func TestIPAVersionWithoutApp(t *testing.T) {
	path := writeIPA(t, map[string]any{"iTunesMetadata.plist": []byte("<plist/>")})

	if _, err := ipaVersion(path); err == nil {
		t.Fatal("ipaVersion succeeded for an IPA with no app")
	}
}

// TestReplicateSinfWritesLicense guards the bug where downloads came back
// without SC_Info/<exec>.sinf: ipatool's Download returns the FairPlay
// licenses but leaves writing them to ReplicateSinf, and an app with no sinf
// can't be launched, so it can't be decrypted.
func TestReplicateSinfWritesLicense(t *testing.T) {
	path := writeIPA(t, map[string]any{
		"Payload/Test.app/Info.plist": map[string]any{"CFBundleExecutable": "Test", "CFBundleShortVersionString": "1.0"},
		"Payload/Test.app/Test":       []byte("not really a mach-o"),
	})

	c, err := New(filepath.Join(t.TempDir(), "cookies"))
	if err != nil {
		t.Fatal(err)
	}

	license := []byte("fairplay-license-bytes")
	if err := c.ipatool().ReplicateSinf(ipatool.ReplicateSinfInput{
		Sinfs:       []ipatool.Sinf{{ID: 0, Data: license}},
		PackagePath: path,
	}); err != nil {
		t.Fatalf("ReplicateSinf: %v", err)
	}

	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if f.Name == "Payload/Test.app/SC_Info/Test.sinf" {
			rc, _ := f.Open()
			got, _ := io.ReadAll(rc)
			rc.Close()
			if string(got) != string(license) {
				t.Fatalf("sinf = %q, want %q", got, license)
			}

			return
		}
	}

	t.Fatal("Payload/Test.app/SC_Info/Test.sinf missing after ReplicateSinf")
}
