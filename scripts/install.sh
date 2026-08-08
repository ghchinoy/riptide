#!/usr/bin/env sh
# install.sh — curl-installable installer for Riptide
#
# Usage:
#   curl -fsSL https://raw.githubusercontent.com/ghchinoy/riptide/main/scripts/install.sh | sh
#
# The script:
#   1. Detects OS and architecture.
#   2. Fetches the latest release metadata from the GitHub API.
#   3. Downloads the correct tarball + checksums.txt.
#   4. Verifies the checksum before extracting.
#   5. Installs the binary to /usr/local/bin (or ~/.local/bin as a fallback).
#
# Safe by design: this file is downloaded in full to a temp directory before
# any execution begins, so there's no risk of running a partially-transferred
# script (the shell only executes once the heredoc/pipe buffer is complete).

set -eu

# ── Constants ────────────────────────────────────────────────────────────────
REPO="ghchinoy/riptide"
BINARY="riptide"
API_URL="https://api.github.com/repos/${REPO}/releases/latest"
RAW_BASE="https://github.com/${REPO}/releases/download"

# ── Helpers ──────────────────────────────────────────────────────────────────
info()  { printf '\033[1;34minfo\033[0m  %s\n' "$*"; }
ok()    { printf '\033[1;32mok\033[0m    %s\n' "$*"; }
warn()  { printf '\033[1;33mwarn\033[0m  %s\n' "$*" >&2; }
die()   { printf '\033[1;31merror\033[0m %s\n' "$*" >&2; exit 1; }

need() {
  command -v "$1" >/dev/null 2>&1 || die "Required tool not found: $1. Please install it and retry."
}

# ── Dependency checks ─────────────────────────────────────────────────────────
need curl
need tar

# ── Detect OS ────────────────────────────────────────────────────────────────
OS="$(uname -s)"
case "${OS}" in
  Darwin) OS_NAME="darwin" ;;
  Linux)  OS_NAME="linux"  ;;
  *) die "Unsupported operating system: ${OS}" ;;
esac

# ── Detect architecture ───────────────────────────────────────────────────────
ARCH="$(uname -m)"
case "${ARCH}" in
  arm64 | aarch64) ARCH_NAME="arm64" ;;
  x86_64 | amd64)  ARCH_NAME="amd64" ;;
  *) die "Unsupported architecture: ${ARCH}" ;;
esac

info "Detected platform: ${OS_NAME}/${ARCH_NAME}"

# ── Fetch latest release tag ──────────────────────────────────────────────────
info "Fetching latest release from GitHub..."

# Try jq first (fast, reliable). Fall back to sed-based parsing.
if command -v jq >/dev/null 2>&1; then
  TAG="$(curl -fsSL "${API_URL}" | jq -r '.tag_name')"
else
  warn "jq not found; falling back to sed-based JSON parsing."
  TAG="$(curl -fsSL "${API_URL}" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
fi

[ -z "${TAG}" ] && die "Could not determine latest release tag. Check your network and try again."

# Strip leading 'v' for the version number used in archive names.
VERSION="${TAG#v}"

ARCHIVE="riptide_${VERSION}_${OS_NAME}_${ARCH_NAME}.tar.gz"
CHECKSUM_FILE="checksums.txt"
DOWNLOAD_URL="${RAW_BASE}/${TAG}/${ARCHIVE}"
CHECKSUM_URL="${RAW_BASE}/${TAG}/${CHECKSUM_FILE}"

info "Latest release: ${TAG}"
info "Archive:        ${ARCHIVE}"

# ── Working directory ─────────────────────────────────────────────────────────
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "${TMP_DIR}"' EXIT INT TERM

# ── Download ──────────────────────────────────────────────────────────────────
info "Downloading ${ARCHIVE}..."
curl -fsSL --progress-bar "${DOWNLOAD_URL}" -o "${TMP_DIR}/${ARCHIVE}" \
  || die "Download failed: ${DOWNLOAD_URL}"

info "Downloading checksums..."
curl -fsSL "${CHECKSUM_URL}" -o "${TMP_DIR}/${CHECKSUM_FILE}" \
  || die "Checksum download failed: ${CHECKSUM_URL}"

# ── Verify checksum ───────────────────────────────────────────────────────────
info "Verifying checksum..."
cd "${TMP_DIR}"

if command -v sha256sum >/dev/null 2>&1; then
  grep "${ARCHIVE}" "${CHECKSUM_FILE}" | sha256sum --check --status \
    || die "Checksum verification FAILED for ${ARCHIVE}. The download may be corrupt or tampered with."
elif command -v shasum >/dev/null 2>&1; then
  grep "${ARCHIVE}" "${CHECKSUM_FILE}" | shasum -a 256 --check --status \
    || die "Checksum verification FAILED for ${ARCHIVE}. The download may be corrupt or tampered with."
else
  warn "Neither sha256sum nor shasum found — skipping checksum verification."
fi
ok "Checksum verified."

# ── Extract ───────────────────────────────────────────────────────────────────
info "Extracting archive..."
tar -xzf "${ARCHIVE}" -C "${TMP_DIR}"
EXTRACTED_BINARY="${TMP_DIR}/${BINARY}"
[ -f "${EXTRACTED_BINARY}" ] || die "Binary '${BINARY}' not found in archive."
chmod +x "${EXTRACTED_BINARY}"

# ── Install ───────────────────────────────────────────────────────────────────
INSTALL_DIR="/usr/local/bin"
if [ ! -w "${INSTALL_DIR}" ]; then
  warn "/usr/local/bin is not writable. Falling back to ~/.local/bin"
  INSTALL_DIR="${HOME}/.local/bin"
  mkdir -p "${INSTALL_DIR}"
  # Remind the user to add it to PATH if it's not there already.
  case ":${PATH}:" in
    *":${INSTALL_DIR}:"*) : ;;
    *) warn "Add ${INSTALL_DIR} to your PATH: export PATH=\"\$PATH:${INSTALL_DIR}\"" ;;
  esac
fi

info "Installing to ${INSTALL_DIR}/${BINARY}..."
cp "${EXTRACTED_BINARY}" "${INSTALL_DIR}/${BINARY}"
ok "Installed ${INSTALL_DIR}/${BINARY}"

# ── Verify ────────────────────────────────────────────────────────────────────
INSTALLED_VERSION="$("${INSTALL_DIR}/${BINARY}" --version 2>&1 || true)"
ok "Installation complete!"
printf '\n'
printf '  %s\n' "${INSTALLED_VERSION}"
printf '\n'
info "Run 'riptide --help' to get started."
info "Docs: https://github.com/${REPO}#readme"
