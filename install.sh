#!/bin/sh
# Installs lazycron, and herdr-core if it is missing:
#
#   curl -fsSL https://raw.githubusercontent.com/swalha1999/lazycron/main/install.sh | sh
#   curl -fsSL .../install.sh | sh -s -- --no-herdr      # lazycron only
#
# herdr-core (hc) is a separate project (https://herdr.dev). lazycron runs
# scheduled jobs inside its terminals when it is present and as plain cron
# jobs when it is not. Opt out with --no-herdr or LAZYCRON_NO_HERDR=1.
set -e

REPO="swalha1999/lazycron"
INSTALL_DIR="/usr/local/bin"
HERDR_CORE_INSTALLER="https://releases.herdr.dev/core/install.sh"

NO_HERDR="${LAZYCRON_NO_HERDR:-0}"
for arg in "$@"; do
  case "$arg" in
    --no-herdr) NO_HERDR=1 ;;
    *) echo "Unknown option: $arg" && exit 1 ;;
  esac
done

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *) echo "Unsupported OS: $OS" && exit 1 ;;
esac

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  aarch64|arm64)  ARCH="arm64" ;;
  *) echo "Unsupported architecture: $ARCH" && exit 1 ;;
esac

# Get latest version
VERSION=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
if [ -z "$VERSION" ]; then
  echo "Failed to fetch latest version"
  exit 1
fi

FILENAME="lazycron_${VERSION#v}_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${FILENAME}"

echo "Installing lazycron ${VERSION} (${OS}/${ARCH})..."

TMPDIR=$(mktemp -d)
trap 'rm -rf "$TMPDIR"' EXIT

curl -fsSL "$URL" -o "${TMPDIR}/${FILENAME}"
tar -xzf "${TMPDIR}/${FILENAME}" -C "$TMPDIR"

if [ -w "$INSTALL_DIR" ]; then
  mv "${TMPDIR}/lazycron" "${INSTALL_DIR}/lazycron"
else
  sudo mv "${TMPDIR}/lazycron" "${INSTALL_DIR}/lazycron"
fi

echo "lazycron ${VERSION} installed to ${INSTALL_DIR}/lazycron"

# herdr-core: installed only when missing, with herdr's own core-only
# installer (no desktop app), so an existing core is never updated or
# restarted from here.
have_herdr_core() {
  command -v herdr-core >/dev/null 2>&1 || [ -x "$HOME/.local/bin/herdr-core" ]
}

if [ "$NO_HERDR" = "1" ]; then
  echo "Skipping herdr-core (--no-herdr). Jobs will run as plain cron jobs."
elif have_herdr_core; then
  echo "herdr-core is already installed; leaving it as it is."
else
  echo
  echo "Installing herdr-core, a separate project (https://herdr.dev), with its official installer."
  echo "lazycron uses it to run scheduled jobs in terminals you can watch and take over."
  if ! curl -fsSL "$HERDR_CORE_INSTALLER" | sh; then
    echo "herdr-core did not install; lazycron still works with plain cron jobs."
    echo "Retry later with: curl -fsSL $HERDR_CORE_INSTALLER | sh"
  fi
fi

echo
if [ "$NO_HERDR" = "1" ]; then
  echo "Next: run 'lazycron' (or 'lazycron agent-help' for the CLI)."
else
  echo "Next: run 'lazycron setup' to start herdr-core at boot and switch jobs to herdr mode"
  echo "(or 'lazycron setup --dry-run' to see what it would change)."
fi
