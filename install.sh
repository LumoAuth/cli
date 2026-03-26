#!/bin/sh
# LumoAuth CLI Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
#
# Installs the lumo CLI to ~/.local/bin without requiring sudo.
# Supports Linux (x86_64, arm64, i386), macOS (x86_64, arm64), and Windows WSL.

set -e

REPO="LumoAuth/cli"
INSTALL_DIR="$HOME/.local/bin"
BINARY_NAME="lumo"

# --- helpers ----------------------------------------------------------------

info()  { printf '  \033[1;34m→\033[0m %s\n' "$*"; }
ok()    { printf '  \033[1;32m✓\033[0m %s\n' "$*"; }
warn()  { printf '  \033[1;33m!\033[0m %s\n' "$*"; }
error() { printf '  \033[1;31m✗\033[0m %s\n' "$*" >&2; exit 1; }

# --- detect OS and architecture ---------------------------------------------

detect_os() {
    case "$(uname -s)" in
        Linux*)  OS="Linux"  ;;
        Darwin*) OS="Darwin" ;;
        *)       error "Unsupported operating system: $(uname -s). Use Linux, macOS or WSL." ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)    ARCH="x86_64" ;;
        aarch64|arm64)   ARCH="arm64"  ;;
        i386|i686)       ARCH="i386"   ;;
        *)               error "Unsupported architecture: $(uname -m). Supported: x86_64, arm64, i386." ;;
    esac
}

# --- resolve latest version -------------------------------------------------

get_latest_version() {
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "https://api.github.com/repos/${REPO}/releases/latest" \
            | grep '"tag_name"' | head -1 | sed 's/.*"tag_name": *"//;s/".*//')
    else
        error "Either curl or wget is required to download the CLI."
    fi

    if [ -z "$VERSION" ]; then
        error "Could not determine the latest release version. Check https://github.com/${REPO}/releases"
    fi
}

# --- download & install -----------------------------------------------------

download() {
    _url="$1"
    _dest="$2"
    if command -v curl >/dev/null 2>&1; then
        curl -fsSL -o "$_dest" "$_url"
    elif command -v wget >/dev/null 2>&1; then
        wget -qO "$_dest" "$_url"
    fi
}

verify_checksums() {
    CHECKSUMS_FILE="cli_${VERSION#v}_checksums.txt"
    CHECKSUMS_URL="https://github.com/${REPO}/releases/download/${VERSION}/${CHECKSUMS_FILE}"

    info "Verifying checksum…"
    if ! download "$CHECKSUMS_URL" "${TMPDIR}/${CHECKSUMS_FILE}" 2>/dev/null; then
        warn "Could not download checksums file — skipping verification."
        return 0
    fi

    EXPECTED=$(grep "${TARBALL}" "${TMPDIR}/${CHECKSUMS_FILE}" | awk '{print $1}')
    if [ -z "$EXPECTED" ]; then
        warn "No checksum found for ${TARBALL} — skipping verification."
        return 0
    fi

    if command -v sha256sum >/dev/null 2>&1; then
        ACTUAL=$(sha256sum "${TMPDIR}/${TARBALL}" | awk '{print $1}')
    elif command -v shasum >/dev/null 2>&1; then
        ACTUAL=$(shasum -a 256 "${TMPDIR}/${TARBALL}" | awk '{print $1}')
    else
        warn "Neither sha256sum nor shasum found — skipping verification."
        return 0
    fi

    if [ "$EXPECTED" != "$ACTUAL" ]; then
        error "Checksum mismatch!\n  Expected: ${EXPECTED}\n  Got:      ${ACTUAL}"
    fi

    ok "Checksum verified."
}

download_binary() {
    TARBALL="cli_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"

    info "Downloading ${BINARY_NAME} ${VERSION} for ${OS}/${ARCH}…"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    if ! download "$URL" "${TMPDIR}/${TARBALL}" 2>/dev/null; then
        error "Download failed. URL: ${URL}"
    fi

    if [ ! -f "${TMPDIR}/${TARBALL}" ] || [ "$(wc -c < "${TMPDIR}/${TARBALL}" 2>/dev/null)" -lt 100 ]; then
        error "Download failed — file is empty or missing. URL: ${URL}"
    fi

    verify_checksums

    info "Extracting…"
    tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

    # GoReleaser puts the binary as 'cli' in the archive
    if [ -f "${TMPDIR}/cli" ]; then
        mv "${TMPDIR}/cli" "${TMPDIR}/${BINARY_NAME}"
    fi

    if [ ! -f "${TMPDIR}/${BINARY_NAME}" ]; then
        error "Archive did not contain a '${BINARY_NAME}' or 'cli' binary."
    fi

    mkdir -p "$INSTALL_DIR"
    mv "${TMPDIR}/${BINARY_NAME}" "${INSTALL_DIR}/${BINARY_NAME}"
    chmod +x "${INSTALL_DIR}/${BINARY_NAME}"

    ok "Installed ${BINARY_NAME} to ${INSTALL_DIR}/${BINARY_NAME}"
}

# --- ensure INSTALL_DIR is on PATH ------------------------------------------

ensure_path() {
    case ":$PATH:" in
        *":${INSTALL_DIR}:"*) return ;;   # already on PATH
    esac

    warn "${INSTALL_DIR} is not in your PATH."

    SHELL_NAME=$(basename "${SHELL:-/bin/sh}")
    case "$SHELL_NAME" in
        bash)
            PROFILE="$HOME/.bashrc"
            ;;
        zsh)
            PROFILE="$HOME/.zshrc"
            ;;
        fish)
            PROFILE="$HOME/.config/fish/config.fish"
            ;;
        *)
            PROFILE="$HOME/.profile"
            ;;
    esac

    EXPORT_LINE="export PATH=\"${INSTALL_DIR}:\$PATH\""

    if [ "$SHELL_NAME" = "fish" ]; then
        EXPORT_LINE="fish_add_path ${INSTALL_DIR}"
    fi

    # only append if not already present
    if [ -f "$PROFILE" ] && grep -qF "$INSTALL_DIR" "$PROFILE" 2>/dev/null; then
        info "PATH entry already exists in ${PROFILE}"
    else
        printf '\n# Added by LumoAuth CLI installer\n%s\n' "$EXPORT_LINE" >> "$PROFILE"
        ok "Added ${INSTALL_DIR} to PATH in ${PROFILE}"
    fi

    info "Restart your shell or run:  source ${PROFILE}"
}

# --- main -------------------------------------------------------------------

main() {
    printf '\n\033[1m  LumoAuth CLI Installer\033[0m\n\n'

    detect_os
    detect_arch
    get_latest_version
    download_binary
    ensure_path

    printf '\n\033[1m  Done! Get started:\033[0m\n\n'
    printf '    # Set up your LumoAuth tenant connection\n'
    printf '    lumo config init\n\n'
    printf '    # Once configured, try:\n'
    printf '    lumo users list\n'
    printf '    lumo roles list\n'
    printf '    lumo settings get auth\n\n'
    printf '  Run \033[1mlumo --help\033[0m for the full list of commands.\n\n'
}

main
