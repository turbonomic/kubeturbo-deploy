#!/bin/sh

# download tools to local bin
LOCALBIN=${LOCALBIN-"bin"}
mkdir -p "${LOCALBIN}"

# Function to generate the download URL for shellcheck
download_shellcheck() {
    version="$1"

    # Detect the operating system for shellcheck
    unset os
    case "$(uname)" in
        Darwin) os="darwin";;
        Linux) os="linux";;
        *) echo "Unsupported OS: $(uname)" && exit 1;;
    esac

    # Detect the architecture for shellcheck
    unset arch
    case "$(uname -m)" in
        x86_64) arch="x86_64";;
        arm64) arch="aarch64";;  # Handle Apple Silicon M1 and M2 architecture
        aarch64) arch="aarch64";;
        armv6l) arch="armv6hf";;
        riscv64) arch="riscv64";;
        *) echo "Unsupported architecture: $(uname -m)" && exit 1;;
    esac

    # Construct the URL
    base_url="https://github.com/koalaman/shellcheck/releases/download/${version}"
    file_name="shellcheck-${version}.${os}.${arch}.tar.xz"
    download_url="${base_url}/${file_name}"

    # Download the shellcheck
    curl -L "$download_url" | tar -xJf - -C "${LOCALBIN}"

    # Extract the tool to local bin
    cp "${LOCALBIN}/shellcheck-${version}/shellcheck" "${LOCALBIN}"

    # Clean up
    rm -rf "${LOCALBIN}/shellcheck-${version}"

    "${LOCALBIN}"/shellcheck --version
}

# Download shellcheck 
SPELLCHECK_VERSION="v0.10.0"
if [ ! -f "${LOCALBIN}"/"shellcheck" ]; then
    download_shellcheck "${SPELLCHECK_VERSION}"
fi
