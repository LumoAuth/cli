// Package upgrade implements self-update for the lumo CLI: install-channel
// detection (Homebrew / Scoop / winget / manual), GitHub release asset
// naming, checksum verification, and atomic binary self-replacement.
package upgrade

import (
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Channel identifies how the running binary was installed.
type Channel int

const (
	// ChannelManual covers install.sh, `go build`, and hand-placed binaries.
	ChannelManual Channel = iota
	// ChannelHomebrew — binary lives in a Homebrew cellar/prefix.
	ChannelHomebrew
	// ChannelScoop — binary lives under a Scoop apps/shims directory.
	ChannelScoop
	// ChannelWinget — binary was installed by the Windows Package Manager.
	ChannelWinget
)

func (c Channel) String() string {
	switch c {
	case ChannelHomebrew:
		return "homebrew"
	case ChannelScoop:
		return "scoop"
	case ChannelWinget:
		return "winget"
	default:
		return "manual"
	}
}

// DetectChannel classifies the install channel from the (symlink-resolved)
// executable path. Matching is case-insensitive and separator-agnostic so
// Windows paths classify identically however they're spelled.
//
//	/opt/homebrew/Cellar/lumo/1.4.0/bin/lumo          → homebrew (Apple Silicon)
//	/usr/local/Cellar/lumo/1.4.0/bin/lumo             → homebrew (Intel mac)
//	/home/linuxbrew/.linuxbrew/Cellar/lumo/...        → homebrew (Linuxbrew)
//	C:\Users\x\scoop\apps\lumo\current\lumo.exe       → scoop
//	C:\Users\x\AppData\Local\Microsoft\WinGet\...     → winget
//	~/.local/bin/lumo                                 → manual
func DetectChannel(exePath string) Channel {
	// filepath.ToSlash only rewrites the host OS's separator, so normalize
	// backslashes explicitly — keeps the classifier host-independent.
	p := strings.ToLower(strings.ReplaceAll(exePath, "\\", "/"))
	switch {
	case strings.Contains(p, "/cellar/"),
		strings.Contains(p, "/homebrew/"),
		strings.Contains(p, "/linuxbrew/"):
		return ChannelHomebrew
	case strings.Contains(p, "/scoop/apps/"),
		strings.Contains(p, "/scoop/shims/"):
		return ChannelScoop
	case strings.Contains(p, "/microsoft/winget/"),
		strings.Contains(p, "/winget/packages/"),
		strings.Contains(p, "/winget/links/"):
		return ChannelWinget
	}
	return ChannelManual
}

// UpgradeCommand returns the package-manager command that upgrades the CLI
// for the given channel, or nil for manual installs.
func UpgradeCommand(c Channel) []string {
	switch c {
	case ChannelHomebrew:
		return []string{"brew", "upgrade", "lumoauth/tap/lumo"}
	case ChannelScoop:
		return []string{"scoop", "update", "lumo"}
	case ChannelWinget:
		return []string{"winget", "upgrade", "--id", "LumoAuth.lumo"}
	}
	return nil
}

// AssetName returns the goreleaser archive name for an OS/arch pair,
// mirroring .goreleaser.yaml's archives.name_template:
//
//	lumo_Darwin_arm64.tar.gz, lumo_Linux_x86_64.tar.gz, lumo_Windows_i386.zip
func AssetName(goos, goarch string) string {
	osName := map[string]string{
		"darwin":  "Darwin",
		"linux":   "Linux",
		"windows": "Windows",
	}[goos]
	if osName == "" {
		osName = strings.ToUpper(goos[:1]) + goos[1:]
	}
	arch := goarch
	switch goarch {
	case "amd64":
		arch = "x86_64"
	case "386":
		arch = "i386"
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("lumo_%s_%s%s", osName, arch, ext)
}

// BinaryName is the name of the CLI binary inside release archives.
func BinaryName(goos string) string {
	if goos == "windows" {
		return "lumo.exe"
	}
	return "lumo"
}

// IsNewer reports whether latest is a strictly newer version than current.
// Handles optional leading "v" and pre-release suffixes ("1.5.0-rc.1").
// Unparseable versions fall back to inequality (so a hotfix retag still
// prompts an upgrade rather than silently comparing equal).
func IsNewer(latest, current string) bool {
	ln, lPre, lOK := parseVersion(latest)
	cn, cPre, cOK := parseVersion(current)
	if !lOK || !cOK {
		return strings.TrimPrefix(latest, "v") != strings.TrimPrefix(current, "v")
	}
	for i := 0; i < 3; i++ {
		if ln[i] != cn[i] {
			return ln[i] > cn[i]
		}
	}
	// Same numerics: a release is newer than a pre-release of itself.
	return cPre != "" && lPre == ""
}

// parseVersion splits "v1.2.3-rc.1" into [1 2 3] and "rc.1".
func parseVersion(v string) (nums [3]int, pre string, ok bool) {
	v = strings.TrimPrefix(strings.TrimSpace(v), "v")
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		pre = v[i+1:]
		v = v[:i]
	}
	parts := strings.Split(v, ".")
	if len(parts) == 0 || len(parts) > 3 {
		return nums, pre, false
	}
	for i, p := range parts {
		n, err := strconv.Atoi(p)
		if err != nil {
			return nums, pre, false
		}
		nums[i] = n
	}
	return nums, pre, true
}

// ExpectedChecksum finds the sha256 for asset in a goreleaser checksums.txt
// ("<hex>  <filename>" lines).
func ExpectedChecksum(checksums []byte, asset string) (string, error) {
	sc := bufio.NewScanner(strings.NewReader(string(checksums)))
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) == 2 && fields[1] == asset {
			return strings.ToLower(fields[0]), nil
		}
	}
	return "", fmt.Errorf("no checksum entry for %s in checksums.txt", asset)
}

// VerifySHA256 checks that the file at path hashes to wantHex.
func VerifySHA256(path, wantHex string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, wantHex) {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", wantHex, got)
	}
	return nil
}

// ExtractBinary pulls binName out of a .tar.gz or .zip release archive into
// destDir and returns the extracted file's path.
func ExtractBinary(archivePath, binName, destDir string) (string, error) {
	out := filepath.Join(destDir, binName)
	switch {
	case strings.HasSuffix(archivePath, ".tar.gz"), strings.HasSuffix(archivePath, ".tgz"):
		if err := extractFromTarGz(archivePath, binName, out); err != nil {
			return "", err
		}
	case strings.HasSuffix(archivePath, ".zip"):
		if err := extractFromZip(archivePath, binName, out); err != nil {
			return "", err
		}
	default:
		return "", fmt.Errorf("unsupported archive format: %s", filepath.Base(archivePath))
	}
	return out, nil
}

func extractFromTarGz(archivePath, binName, out string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag == tar.TypeReg && filepath.Base(hdr.Name) == binName {
			return writeFile(out, tr, 0o755)
		}
	}
	return fmt.Errorf("archive does not contain %q", binName)
}

func extractFromZip(archivePath, binName, out string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer zr.Close()
	for _, zf := range zr.File {
		if zf.FileInfo().IsDir() || filepath.Base(zf.Name) != binName {
			continue
		}
		rc, err := zf.Open()
		if err != nil {
			return err
		}
		err = writeFile(out, rc, 0o755)
		rc.Close()
		return err
	}
	return fmt.Errorf("archive does not contain %q", binName)
}

func writeFile(path string, r io.Reader, mode os.FileMode) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, mode)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, r); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// Replace atomically swaps the running executable at exePath with the binary
// at newBinPath.
//
// Unix: the new binary is written to a temp file in the same directory
// (same filesystem → rename is atomic) and renamed over the old one; the
// running process keeps executing its unlinked inode.
//
// Windows: the OS forbids overwriting a running .exe but allows renaming it,
// so the standard two-rename dance is used: current → lumo.exe.old, staged →
// lumo.exe. If even the first rename is blocked (rare: AV/file locks), the
// new binary is left next to the old one as lumo.exe.new and staged=true is
// returned so the caller can print swap instructions.
func Replace(exePath, newBinPath string) (staged bool, err error) {
	dir := filepath.Dir(exePath)

	// Stage the new binary on the destination filesystem first.
	tmp, err := os.CreateTemp(dir, ".lumo-upgrade-*")
	if err != nil {
		return false, fmt.Errorf("cannot write to %s (try re-running with elevated permissions, or upgrade via your package manager): %w", dir, err)
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath) // no-op after successful rename

	src, err := os.Open(newBinPath)
	if err != nil {
		tmp.Close()
		return false, err
	}
	_, copyErr := io.Copy(tmp, src)
	src.Close()
	if closeErr := tmp.Close(); copyErr == nil {
		copyErr = closeErr
	}
	if copyErr != nil {
		return false, copyErr
	}
	if err := os.Chmod(tmpPath, 0o755); err != nil {
		return false, err
	}

	if isWindows(exePath) {
		oldPath := exePath + ".old"
		_ = os.Remove(oldPath) // clean leftover from a previous upgrade
		if err := os.Rename(exePath, oldPath); err != nil {
			// Can't move the running exe aside — stage as .new instead.
			newPath := exePath + ".new"
			_ = os.Remove(newPath)
			if err2 := os.Rename(tmpPath, newPath); err2 != nil {
				return false, fmt.Errorf("stage new binary: %w", err2)
			}
			return true, nil
		}
		if err := os.Rename(tmpPath, exePath); err != nil {
			// Roll back so the user still has a working binary.
			_ = os.Rename(oldPath, exePath)
			return false, fmt.Errorf("install new binary: %w", err)
		}
		// Best-effort: removing the old exe can fail while it's still the
		// running process image; Windows cleans it up on next upgrade.
		_ = os.Remove(oldPath)
		return false, nil
	}

	if err := os.Rename(tmpPath, exePath); err != nil {
		return false, fmt.Errorf("install new binary: %w", err)
	}
	return false, nil
}

// isWindows keys off the executable suffix rather than runtime.GOOS so the
// rename strategy is unit-testable cross-platform.
func isWindows(exePath string) bool {
	return strings.EqualFold(filepath.Ext(exePath), ".exe")
}
