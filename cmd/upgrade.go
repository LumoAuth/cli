package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/lumoauth/cli/internal/upgrade"
	"github.com/spf13/cobra"
)

// `lumo upgrade` — checks GitHub Releases for a newer version, detects how
// this binary was installed (Homebrew cellar / Scoop dir / winget / manual),
// and upgrades through the same channel:
//
//   - brew/scoop/winget installs: prints the package manager's upgrade
//     command and (with --exec, or on TTY confirmation) runs it, so the
//     manager's own state stays consistent.
//   - manual installs (install.sh / hand-placed): downloads the release
//     archive for this OS/arch, verifies it against checksums.txt, and
//     atomically self-replaces the binary (staged .new + instructions on
//     Windows when the running exe can't be moved aside).
//
// `--check` preserves the old report-only behavior.

const (
	upgradeReleasesURL  = "https://api.github.com/repos/lumoauth/cli/releases/latest"
	upgradeDownloadBase = "https://github.com/lumoauth/cli/releases/download"
)

var (
	upgradeCheckOnly bool
	upgradeExec      bool
)

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Upgrade the lumo CLI to the latest release",
	Long: `Checks GitHub Releases for the newest published version, detects the
install channel (Homebrew, Scoop, winget, or a manual install), and
upgrades in place.

Package-manager installs are upgraded through the package manager
(printed, and executed with --exec or after confirmation). Manual
installs are self-updated: the release archive for your OS/arch is
downloaded, verified against the release's checksums.txt, and swapped
in atomically.

Use --check to only report whether a newer version exists.
Exits 0 if up-to-date or upgraded; 2 if a network/parse error prevented
the check.`,
	RunE: runUpgrade,
}

func init() {
	upgradeCmd.Flags().BoolVar(&upgradeCheckOnly, "check", false, "Only check for a newer version; don't upgrade")
	upgradeCmd.Flags().BoolVar(&upgradeExec, "exec", false, "Upgrade without prompting for confirmation")
	rootCmd.AddCommand(upgradeCmd)
}

func runUpgrade(cmd *cobra.Command, args []string) error {
	current := Version
	if current == "" || current == "dev" {
		fmt.Fprintln(os.Stderr,
			"This binary was built without a version stamp. Install a release build to use `lumo upgrade`.")
		return nil
	}

	latest, tag, url, err := fetchLatestRelease()
	if err != nil {
		fmt.Fprintf(os.Stderr, "could not check for updates: %v\n", err)
		os.Exit(2)
	}

	if !upgrade.IsNewer(latest, current) {
		fmt.Printf("lumo is up to date (%s).\n", current)
		return nil
	}

	channel := detectInstallChannel()

	fmt.Printf("A newer release is available: %s (you have %s)\n", latest, current)
	if url != "" {
		fmt.Printf("Release notes: %s\n", url)
	}
	fmt.Println()

	if upgradeCheckOnly {
		printUpgradeInstructions(channel)
		return nil
	}

	if pmCmd := upgrade.UpgradeCommand(channel); pmCmd != nil {
		return upgradeViaPackageManager(channel, pmCmd)
	}
	return selfUpdate(tag, latest)
}

// detectInstallChannel resolves the running executable (following symlinks —
// Homebrew links Cellar binaries into <prefix>/bin) and classifies it.
func detectInstallChannel() upgrade.Channel {
	exe, err := os.Executable()
	if err != nil {
		return upgrade.ChannelManual
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}
	return upgrade.DetectChannel(exe)
}

// printUpgradeInstructions prints the channel-appropriate upgrade command
// without running anything (the --check path).
func printUpgradeInstructions(channel upgrade.Channel) {
	if pmCmd := upgrade.UpgradeCommand(channel); pmCmd != nil {
		fmt.Printf("Installed via %s. Upgrade with:\n", channel)
		fmt.Printf("  %s\n", strings.Join(pmCmd, " "))
		return
	}
	fmt.Println("Upgrade with:")
	fmt.Println("  lumo upgrade                        # self-update in place")
	fmt.Println("  curl -sSL https://lumoauth.dev/install.sh | sh")
}

// upgradeViaPackageManager prints the package manager command and runs it
// with --exec or after an interactive confirmation. Non-TTY without --exec
// only prints (the CLI's no-surprise-interactivity convention for agents).
func upgradeViaPackageManager(channel upgrade.Channel, pmCmd []string) error {
	fmt.Printf("Installed via %s. Upgrade command:\n", channel)
	fmt.Printf("  %s\n", strings.Join(pmCmd, " "))
	fmt.Println()

	if !upgradeExec {
		if !isInteractive() {
			fmt.Println("Re-run with --exec to run it now.")
			return nil
		}
		if !isYes(promptString("Run it now? [y/N]: ")) {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	fmt.Fprintf(os.Stderr, "→ Running %s\n", strings.Join(pmCmd, " "))
	c := exec.Command(pmCmd[0], pmCmd[1:]...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	c.Stdin = os.Stdin
	if err := c.Run(); err != nil {
		return fmt.Errorf("%s failed: %w", pmCmd[0], err)
	}
	return nil
}

// selfUpdate downloads the release archive for this OS/arch, verifies its
// sha256 against the release's checksums.txt, and swaps the binary in place.
func selfUpdate(tag, latest string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate current executable: %w", err)
	}
	if resolved, err := filepath.EvalSymlinks(exe); err == nil {
		exe = resolved
	}

	if !upgradeExec {
		if !isInteractive() {
			fmt.Printf("Would replace %s. Re-run with --exec to upgrade now.\n", exe)
			return nil
		}
		if !isYes(promptString(fmt.Sprintf("Download %s and replace %s? [y/N]: ", latest, exe))) {
			fmt.Fprintln(os.Stderr, "Aborted.")
			return nil
		}
	}

	asset := upgrade.AssetName(runtime.GOOS, runtime.GOARCH)
	binName := upgrade.BinaryName(runtime.GOOS)

	tmpDir, err := os.MkdirTemp("", "lumo-upgrade-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	fmt.Fprintf(os.Stderr, "→ Downloading %s %s…\n", asset, latest)
	archivePath := filepath.Join(tmpDir, asset)
	if err := downloadReleaseAsset(tag, asset, archivePath); err != nil {
		return fmt.Errorf("download %s: %w", asset, err)
	}

	fmt.Fprintln(os.Stderr, "→ Verifying checksum…")
	checksumsPath := filepath.Join(tmpDir, "checksums.txt")
	if err := downloadReleaseAsset(tag, "checksums.txt", checksumsPath); err != nil {
		return fmt.Errorf("download checksums.txt: %w", err)
	}
	sums, err := os.ReadFile(checksumsPath)
	if err != nil {
		return err
	}
	want, err := upgrade.ExpectedChecksum(sums, asset)
	if err != nil {
		return err
	}
	if err := upgrade.VerifySHA256(archivePath, want); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Checksum verified.")

	newBin, err := upgrade.ExtractBinary(archivePath, binName, tmpDir)
	if err != nil {
		return err
	}

	staged, err := upgrade.Replace(exe, newBin)
	if err != nil {
		return err
	}
	if staged {
		fmt.Printf("The running executable could not be replaced while in use.\n")
		fmt.Printf("The new version was staged at:\n  %s.new\n", exe)
		fmt.Printf("Finish the upgrade with:\n")
		fmt.Printf("  move /y \"%s.new\" \"%s\"\n", exe, exe)
		return nil
	}
	fmt.Printf("✓ Upgraded lumo to %s (%s)\n", latest, exe)
	return nil
}

// downloadReleaseAsset fetches a named asset of the tagged GitHub release.
func downloadReleaseAsset(tag, asset, dest string) error {
	url := fmt.Sprintf("%s/%s/%s", upgradeDownloadBase, tag, asset)
	hc := &http.Client{Timeout: 5 * time.Minute}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "lumo-cli/"+Version)
	resp, err := hc.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s returned HTTP %d", url, resp.StatusCode)
	}
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		return err
	}
	return f.Close()
}

// fetchLatestRelease queries the GitHub Releases API for the newest version.
// Returns the bare version (tag with any leading 'v' stripped), the raw tag
// (used to build download URLs), and the release page URL.
func fetchLatestRelease() (version, tag, htmlURL string, err error) {
	hc := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", upgradeReleasesURL, nil)
	if err != nil {
		return "", "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lumo-cli/"+Version)

	resp, err := hc.Do(req)
	if err != nil {
		return "", "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", "", fmt.Errorf("github releases API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", "", err
	}
	// GitHub tags conventionally prefix with 'v'; strip it for compare.
	return strings.TrimPrefix(release.TagName, "v"), release.TagName, release.HTMLURL, nil
}
