#!/usr/bin/env bash
#
# setup-dev.sh — provision a development environment for astql.
#
# Installs the Go toolchain pinned by go.mod, a C compiler (required by the
# race detector that `make test` runs), the project's lint/security tooling,
# and the module dependencies. Safe to run repeatedly.
#
# Usage:
#   ./tools/setup-dev.sh
#
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GO_INSTALL_DIR="/usr/local/go"

log()  { printf '\033[36m==>\033[0m %s\n' "$*"; }
warn() { printf '\033[33mwarning:\033[0m %s\n' "$*" >&2; }
die()  { printf '\033[31merror:\033[0m %s\n' "$*" >&2; exit 1; }

# --- Resolve the Go version pinned by go.mod ---------------------------------
# Prefer the `toolchain` directive (what CI builds with); fall back to `go`.
resolve_go_version() {
	local v
	v="$(grep -E '^toolchain go' "$REPO_ROOT/go.mod" 2>/dev/null | awk '{print $2}' | sed 's/^go//')"
	if [ -z "$v" ]; then
		v="$(grep -E '^go [0-9]' "$REPO_ROOT/go.mod" 2>/dev/null | awk '{print $2}')"
	fi
	[ -n "$v" ] || die "could not determine Go version from go.mod"
	printf '%s' "$v"
}

# --- Map uname -m to a Go release arch ---------------------------------------
resolve_arch() {
	case "$(uname -m)" in
		x86_64|amd64)   printf 'amd64' ;;
		aarch64|arm64)  printf 'arm64' ;;
		*) die "unsupported architecture: $(uname -m)" ;;
	esac
}

# --- Install Go if missing or the wrong version ------------------------------
install_go() {
	local want="$1" arch os tarball url current
	os="$(uname -s | tr '[:upper:]' '[:lower:]')"
	arch="$(resolve_arch)"

	if [ -x "$GO_INSTALL_DIR/bin/go" ]; then
		current="$("$GO_INSTALL_DIR/bin/go" version | awk '{print $3}' | sed 's/^go//')"
		if [ "$current" = "$want" ]; then
			log "Go $want already installed at $GO_INSTALL_DIR"
			return
		fi
		log "Replacing Go $current with $want"
	fi

	tarball="go${want}.${os}-${arch}.tar.gz"
	url="https://go.dev/dl/${tarball}"
	log "Downloading $url"
	local tmp
	tmp="$(mktemp -d)"
	if ! curl -fsSL --max-time 180 "$url" -o "$tmp/$tarball"; then
		rm -rf "$tmp"
		die "failed to download Go from $url"
	fi
	log "Installing Go to $GO_INSTALL_DIR (requires write access)"
	rm -rf "$GO_INSTALL_DIR"
	tar -C "$(dirname "$GO_INSTALL_DIR")" -xzf "$tmp/$tarball"
	rm -rf "$tmp"
	"$GO_INSTALL_DIR/bin/go" version
}

# --- Install a C compiler (needed for `go test -race`) -----------------------
install_cc() {
	if command -v cc >/dev/null 2>&1 || command -v gcc >/dev/null 2>&1; then
		log "C compiler already present"
		return
	fi
	log "Installing C compiler (needed by the race detector)"
	local sudo=""
	[ "$(id -u)" -ne 0 ] && command -v sudo >/dev/null 2>&1 && sudo="sudo"
	if command -v apt-get >/dev/null 2>&1; then
		$sudo apt-get update -qq && $sudo apt-get install -y -qq build-essential
	elif command -v dnf >/dev/null 2>&1; then
		$sudo dnf install -y -q gcc
	elif command -v yum >/dev/null 2>&1; then
		$sudo yum install -y -q gcc
	elif command -v pacman >/dev/null 2>&1; then
		$sudo pacman -Sy --noconfirm gcc
	elif command -v apk >/dev/null 2>&1; then
		$sudo apk add --no-cache build-base
	else
		warn "no known package manager found; install gcc manually or 'make test' (race) will fail"
		return
	fi
}

# --- Persist PATH for future shells ------------------------------------------
persist_path() {
	local gobin profile="/etc/profile.d/go.sh"
	gobin="$("$GO_INSTALL_DIR/bin/go" env GOPATH)/bin"
	if [ -w "$(dirname "$profile")" ] 2>/dev/null; then
		printf 'export PATH="%s/bin:%s:$PATH"\n' "$GO_INSTALL_DIR" "$gobin" > "$profile"
		log "Wrote PATH to $profile (applies to new shells)"
	else
		warn "cannot write $profile; add this to your shell profile manually:"
		printf '    export PATH="%s/bin:%s:$PATH"\n' "$GO_INSTALL_DIR" "$gobin" >&2
	fi
}

main() {
	local go_version
	go_version="$(resolve_go_version)"
	log "astql dev setup — target Go $go_version"

	install_go "$go_version"
	install_cc

	export PATH="$GO_INSTALL_DIR/bin:$($GO_INSTALL_DIR/bin/go env GOPATH)/bin:$PATH"
	# go.mod pins a toolchain; we install it directly, so don't let Go fetch another.
	export GOTOOLCHAIN=local

	log "Downloading module dependencies"
	( cd "$REPO_ROOT" && go mod download )

	log "Installing lint/security tooling (make install-tools)"
	( cd "$REPO_ROOT" && make install-tools )

	persist_path

	log "Done. Verify with:  make check"
	printf '\nOpen a new shell (or run the export above) so go and the tools are on PATH.\n'
}

main "$@"
