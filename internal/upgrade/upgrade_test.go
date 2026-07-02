package upgrade

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

func TestDetectChannel(t *testing.T) {
	cases := []struct {
		path string
		want Channel
	}{
		// Homebrew
		{"/opt/homebrew/Cellar/lumo/1.4.0/bin/lumo", ChannelHomebrew},
		{"/opt/homebrew/bin/lumo", ChannelHomebrew},
		{"/usr/local/Cellar/lumo/1.4.0/bin/lumo", ChannelHomebrew},
		{"/home/linuxbrew/.linuxbrew/Cellar/lumo/1.4.0/bin/lumo", ChannelHomebrew},
		{"/home/linuxbrew/.linuxbrew/bin/lumo", ChannelHomebrew},
		// Scoop (shim and real path, either separator, any casing)
		{`C:\Users\jane\scoop\apps\lumo\current\lumo.exe`, ChannelScoop},
		{`C:\Users\jane\scoop\shims\lumo.exe`, ChannelScoop},
		{"C:/Users/jane/Scoop/Apps/lumo/1.4.0/lumo.exe", ChannelScoop},
		// winget portable installs + links
		{`C:\Users\jane\AppData\Local\Microsoft\WinGet\Packages\LumoAuth.lumo_x\lumo.exe`, ChannelWinget},
		{`C:\Users\jane\AppData\Local\Microsoft\WinGet\Links\lumo.exe`, ChannelWinget},
		// Manual
		{"/home/jane/.local/bin/lumo", ChannelManual},
		{"/usr/local/bin/lumo", ChannelManual},
		{`C:\tools\lumo.exe`, ChannelManual},
		{"/Users/jane/src/cli/lumo", ChannelManual},
	}
	for _, tc := range cases {
		if got := DetectChannel(tc.path); got != tc.want {
			t.Errorf("DetectChannel(%q) = %s, want %s", tc.path, got, tc.want)
		}
	}
}

func TestUpgradeCommand(t *testing.T) {
	if cmd := UpgradeCommand(ChannelManual); cmd != nil {
		t.Errorf("manual channel should have no package-manager command, got %v", cmd)
	}
	for _, c := range []Channel{ChannelHomebrew, ChannelScoop, ChannelWinget} {
		if cmd := UpgradeCommand(c); len(cmd) < 2 {
			t.Errorf("UpgradeCommand(%s) = %v", c, cmd)
		}
	}
}

func TestAssetName(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "lumo_Darwin_arm64.tar.gz"},
		{"darwin", "amd64", "lumo_Darwin_x86_64.tar.gz"},
		{"linux", "amd64", "lumo_Linux_x86_64.tar.gz"},
		{"linux", "386", "lumo_Linux_i386.tar.gz"},
		{"linux", "arm64", "lumo_Linux_arm64.tar.gz"},
		{"windows", "amd64", "lumo_Windows_x86_64.zip"},
		{"windows", "386", "lumo_Windows_i386.zip"},
	}
	for _, tc := range cases {
		if got := AssetName(tc.goos, tc.goarch); got != tc.want {
			t.Errorf("AssetName(%s,%s) = %q, want %q", tc.goos, tc.goarch, got, tc.want)
		}
	}
}

func TestIsNewer(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"1.5.0", "1.4.0", true},
		{"v1.5.0", "1.4.9", true},
		{"1.4.0", "1.4.0", false},
		{"1.4.0", "1.5.0", false},
		{"2.0.0", "1.99.99", true},
		{"1.10.0", "1.9.0", true}, // numeric, not lexicographic
		{"1.5.0", "1.5.0-rc.1", true},
		{"1.5.0-rc.1", "1.5.0", false},
		{"weird-tag", "1.4.0", true}, // unparseable → fall back to inequality
		{"1.4", "1.4.0", false},      // short form == padded form
	}
	for _, tc := range cases {
		if got := IsNewer(tc.latest, tc.current); got != tc.want {
			t.Errorf("IsNewer(%q, %q) = %v, want %v", tc.latest, tc.current, got, tc.want)
		}
	}
}

func TestExpectedChecksumAndVerify(t *testing.T) {
	dir := t.TempDir()
	payload := []byte("fake binary payload")
	asset := filepath.Join(dir, "lumo_Linux_x86_64.tar.gz")
	if err := os.WriteFile(asset, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(payload)
	hexSum := hex.EncodeToString(sum[:])

	checksums := []byte(
		"deadbeef  lumo_Darwin_arm64.tar.gz\n" +
			hexSum + "  lumo_Linux_x86_64.tar.gz\n")

	got, err := ExpectedChecksum(checksums, "lumo_Linux_x86_64.tar.gz")
	if err != nil {
		t.Fatal(err)
	}
	if got != hexSum {
		t.Errorf("ExpectedChecksum = %q, want %q", got, hexSum)
	}
	if _, err := ExpectedChecksum(checksums, "lumo_Windows_i386.zip"); err == nil {
		t.Error("missing asset should error")
	}

	if err := VerifySHA256(asset, hexSum); err != nil {
		t.Errorf("VerifySHA256 valid: %v", err)
	}
	if err := VerifySHA256(asset, "deadbeef"); err == nil {
		t.Error("VerifySHA256 should reject a wrong checksum")
	}
}

func writeTarGz(t *testing.T, path, binName string, content []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	for _, e := range []struct {
		name string
		body []byte
	}{
		{"README.md", []byte("readme")},
		{binName, content},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: e.name, Mode: 0o755, Size: int64(len(e.body))}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(e.body); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
}

func writeZip(t *testing.T, path, binName string, content []byte) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zw := zip.NewWriter(f)
	w, err := zw.Create(binName)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(content); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestExtractBinaryTarGz(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "lumo_Linux_x86_64.tar.gz")
	writeTarGz(t, archive, "lumo", []byte("elf!"))

	out, err := ExtractBinary(archive, "lumo", dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "elf!" {
		t.Errorf("extracted content = %q", data)
	}
}

func TestExtractBinaryZip(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "lumo_Windows_x86_64.zip")
	writeZip(t, archive, "lumo.exe", []byte("mz!"))

	out, err := ExtractBinary(archive, "lumo.exe", dir)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "mz!" {
		t.Errorf("extracted content = %q", data)
	}
}

func TestExtractBinaryMissing(t *testing.T) {
	dir := t.TempDir()
	archive := filepath.Join(dir, "lumo_Linux_x86_64.tar.gz")
	writeTarGz(t, archive, "not-lumo", []byte("x"))
	if _, err := ExtractBinary(archive, "lumo", dir); err == nil {
		t.Error("expected error when binary missing from archive")
	}
}

func TestReplaceUnix(t *testing.T) {
	dir := t.TempDir()
	exe := filepath.Join(dir, "lumo")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "lumo-new")
	if err := os.WriteFile(newBin, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	staged, err := Replace(exe, newBin)
	if err != nil {
		t.Fatal(err)
	}
	if staged {
		t.Error("unix replace should not stage")
	}
	data, err := os.ReadFile(exe)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new" {
		t.Errorf("binary content = %q, want new", data)
	}
	info, _ := os.Stat(exe)
	if perm := info.Mode().Perm(); perm&0o111 == 0 {
		t.Errorf("replaced binary not executable: %o", perm)
	}
	// No stray temp files left behind.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if e.Name() != "lumo" && e.Name() != "lumo-new" {
			t.Errorf("leftover file after replace: %s", e.Name())
		}
	}
}

func TestReplaceWindowsStyleTwoRename(t *testing.T) {
	// .exe suffix routes through the Windows two-rename strategy; on a
	// POSIX fs both renames succeed, exercising the happy path.
	dir := t.TempDir()
	exe := filepath.Join(dir, "lumo.exe")
	if err := os.WriteFile(exe, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	newBin := filepath.Join(dir, "incoming.bin")
	if err := os.WriteFile(newBin, []byte("new"), 0o600); err != nil {
		t.Fatal(err)
	}

	staged, err := Replace(exe, newBin)
	if err != nil {
		t.Fatal(err)
	}
	if staged {
		t.Error("two-rename happy path should not stage")
	}
	data, _ := os.ReadFile(exe)
	if string(data) != "new" {
		t.Errorf("binary content = %q, want new", data)
	}
}
