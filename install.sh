#!/bin/sh
# LumoAuth CLI Installer
# Usage: curl -fsSL https://raw.githubusercontent.com/LumoAuth/cli/main/install.sh | sh
#
# Installs the lumo CLI to ~/.local/bin without requiring sudo.
# Supports Linux (amd64, arm64), macOS (amd64, arm64), and Windows WSL.

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
        Linux*)  OS="linux"  ;;
        Darwin*) OS="darwin" ;;
        *)       error "Unsupported operating system: $(uname -s). Use Linux, macOS or WSL." ;;
    esac
}

detect_arch() {
    case "$(uname -m)" in
        x86_64|amd64)   ARCH="amd64"  ;;
        aarch64|arm64)   ARCH="arm64"  ;;
        *)               error "Unsupported architecture: $(uname -m). Use amd64 or arm64." ;;
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

download_binary() {
    TARBALL="lumo_${VERSION#v}_${OS}_${ARCH}.tar.gz"
    URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"

    info "Downloading ${BINARY_NAME} ${VERSION} for ${OS}/${ARCH}…"

    TMPDIR=$(mktemp -d)
    trap 'rm -rf "$TMPDIR"' EXIT

    if command -v curl >/dev/null 2>&1; then
        HTTP_CODE=$(curl -fsSL -w '%{http_code}' -o "${TMPDIR}/${TARBALL}" "$URL" 2>/dev/null) || true
    elif command -v wget >/dev/null 2>&1; then
        HTTP_CODE=$(wget --server-response -qO "${TMPDIR}/${TARBALL}" "$URL" 2>&1 | awk '/^  HTTP/{print $2}' | tail -1) || true
    fi

    if [ ! -f "${TMPDIR}/${TARBALL}" ] || [ "$(wc -c < "${TMPDIR}/${TARBALL}" 2>/dev/null)" -lt 100 ]; then
        error "Download failed (HTTP ${HTTP_CODE:-???}). URL: ${URL}"
    fi

    info "Extracting…"
    tar -xzf "${TMPDIR}/${TARBALL}" -C "$TMPDIR"

    if [ ! -f "${TMPDIR}/${BINARY_NAME}" ]; then
        error "Archive did not contain a '${BINARY_NAME}' binary."
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
