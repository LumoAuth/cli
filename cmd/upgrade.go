package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"time"

	"github.com/spf13/cobra"
)

// `lumo upgrade` — checks GitHub Releases for a newer version. We don't
// in-place replace the binary (the install script and Homebrew formula
// own that lifecycle); we tell the user how to update with the same
// mechanism they used to install. This is the same pattern `gh` and
// `stripe` follow.

const upgradeReleasesURL = "https://api.github.com/repos/lumoauth/cli/releases/latest"

var upgradeCmd = &cobra.Command{
	Use:   "upgrade",
	Short: "Check for a newer release of the lumo CLI",
	Long: `Queries GitHub Releases for the newest published version and
compares against the binary you have. Exits 0 if up-to-date or a new
version is available; 2 if a network/parse error prevented the check.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		current := Version
		if current == "" || current == "dev" {
			fmt.Fprintln(os.Stderr,
				"This binary was built without a version stamp. Install a release build to use `lumo upgrade`.")
			return nil
		}

		latest, url, err := fetchLatestRelease()
		if err != nil {
			fmt.Fprintf(os.Stderr, "could not check for updates: %v\n", err)
			os.Exit(2)
		}

		if latest == current {
			fmt.Printf("lumo is up to date (%s).\n", current)
			return nil
		}

		fmt.Printf("A newer release is available: %s (you have %s)\n", latest, current)
		fmt.Println()
		fmt.Println("Update with:")
		switch runtime.GOOS {
		case "darwin":
			fmt.Println("  brew upgrade lumoauth/lumo/lumo      # if installed via Homebrew")
			fmt.Println("  curl -sSL https://lumoauth.com/install.sh | sh")
		case "linux":
			fmt.Println("  curl -sSL https://lumoauth.com/install.sh | sh")
		case "windows":
			fmt.Println("  scoop update lumo                    # if installed via Scoop")
			fmt.Println("  winget upgrade --id LumoAuth.lumo    # if installed via winget")
		default:
			fmt.Println("  curl -sSL https://lumoauth.com/install.sh | sh")
		}
		fmt.Println()
		if url != "" {
			fmt.Printf("Release notes: %s\n", url)
		}
		return nil
	},
}

func fetchLatestRelease() (version string, htmlURL string, err error) {
	hc := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", upgradeReleasesURL, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "lumo-cli/"+Version)

	resp, err := hc.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("github releases API returned HTTP %d", resp.StatusCode)
	}

	var release struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", "", err
	}
	// GitHub tags conventionally prefix with 'v'; strip it for compare.
	tag := release.TagName
	if len(tag) > 0 && tag[0] == 'v' {
		tag = tag[1:]
	}
	return tag, release.HTMLURL, nil
}

func init() {
	rootCmd.AddCommand(upgradeCmd)
}
