package appstore

import (
	"archive/zip"
	"os"
	"path/filepath"
	"testing"

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
