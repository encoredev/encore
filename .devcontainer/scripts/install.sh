#!/usr/bin/env bash
set -e

encore_uri=$(curl -sSf -N "https://encore.dev/api/releases?target=linux_amd64&show=url")
if [ ! "$encore_uri" ]; then
    echo "Error: Unable to determine latest Encore release." 1>&2
    exit 1
fi

encore_install="/encore-release"
bin_dir="$encore_install/bin"
exe="$bin_dir/encore"
tar="$encore_install/encore.tar.gz"

if [ ! -d "$bin_dir" ]; then
 	mkdir -p "$bin_dir"
fi

curl --fail --location --progress-bar --output "$tar" "$encore_uri"
cd "$encore_install"
tar -C "$encore_install" -xzf "$tar"
chmod +x "$bin_dir"/*
rm "$tar"

"$exe" version

echo "Encore was installed successfully to $exe"
if command -v encore >/dev/null; then
	echo "Run 'encore --help' to get started"
else
	case $SHELL in
	/bin/zsh) shell_profile=".zshrc" ;;
	*) shell_profile=".bash_profile" ;;
	esac
	echo "Manually add the directory to your \$HOME/$shell_profile (or similar)"
	echo "  export ENCORE_INSTALL=\"$encore_install\""
	echo "  export PATH=\"\$ENCORE_INSTALL/bin:\$PATH\""
	echo "Run '$exe --help' to get started"
fi