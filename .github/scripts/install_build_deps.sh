#!/bin/sh

set -e

DIR=$(cd -P -- "$(dirname -- "$(command -v -- "$0")")" && pwd -P)

cd "${DIR}/.."

TEMPL_VERSION="v0.3.943"
TAILWINDCSS_VERSION="v4.1.11"

arch=$(uname -m)
if [ "${arch}" = "x86_64" ]; then
	tailwindcss_arch="x64"
elif [ "${arch}" = "aarch64" ]; then
	tailwindcss_arch=arm64
else
	echo "Unsupported architecture: ${arch}"
	exit 1
fi
os=$(uname -s)
if [ "${os}" = "Darwin" ]; then
	tailwindcss_os=macos
elif [ "${os}" = "Linux" ]; then
	tailwindcss_os=linux
else
	echo "Unsupported OS: ${os}"
	exit 1
fi

install_templ() {
	# Install templ if not already installed
	if [ "${FORCE}" -eq 1 ] || ! command -v templ >/dev/null 2>&1; then
		echo "Installing templ with version ${TEMPL_VERSION}"
		go install github.com/a-h/templ/cmd/templ@"${TEMPL_VERSION}"
	fi
}

install_tailwindcss() {
	# Install tailwindcss if not already installed
	if [ "${FORCE}" -eq 1 ] || ! command -v tailwindcss >/dev/null 2>&1; then
		echo "Installing tailwindcss with version ${TAILWINDCSS_VERSION}"
		tailwindcss_url="https://github.com/tailwindlabs/tailwindcss/releases/download/${TAILWINDCSS_VERSION}/tailwindcss-${tailwindcss_os}-${tailwindcss_arch}"
		curl -sLO "${tailwindcss_url}"
		chmod +x "tailwindcss-${tailwindcss_os}-${tailwindcss_arch}"
		sudo mv "tailwindcss-${tailwindcss_os}-${tailwindcss_arch}" /usr/local/bin/tailwindcss
	fi
}

#flags
FORCE=0

while getopts "f" opt; do
	case ${opt} in
		f)
			FORCE=1
			;;
		\?)
			echo "Invalid option: $OPTARG" 1>&2
			exit 1
			;;
	esac
done

install_templ
install_tailwindcss

echo "Installed templ version: $(templ version)"
echo "Installed tailwindcss version: $(tailwindcss version)"
