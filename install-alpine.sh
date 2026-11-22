#!/bin/sh

set -eu

REPOSITORY="EducLecomte/hollow"
BINARY_NAME="hollow"
INSTALL_DIR=${INSTALL_DIR:-/usr/local/bin}

case "$(uname -m)" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *)
        echo "Architecture non supportée : $(uname -m)" >&2
        echo "Architectures disponibles : x86_64 et aarch64" >&2
        exit 1
        ;;
esac

if [ -n "${1:-}" ]; then
    VERSION="$1"
    case "$VERSION" in
        v*) ;;
        *) VERSION="v$VERSION" ;;
    esac
else
    API_URL="https://api.github.com/repos/$REPOSITORY/releases/latest"
    if command -v curl >/dev/null 2>&1; then
        VERSION=$(curl -fsSL "$API_URL" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
    elif command -v wget >/dev/null 2>&1; then
        VERSION=$(wget -qO- "$API_URL" | sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p')
    else
        echo "Installez curl ou wget avec : apk add --no-cache curl" >&2
        exit 1
    fi

    if [ -z "$VERSION" ]; then
        echo "Impossible de déterminer la dernière version. Indiquez un tag, par exemple : $0 v1.2.0" >&2
        exit 1
    fi
fi

DOWNLOAD_URL="https://github.com/$REPOSITORY/releases/download/$VERSION/$BINARY_NAME-linux-$ARCH"
TEMP_FILE=$(mktemp)
trap 'rm -f "$TEMP_FILE"' EXIT HUP INT TERM

echo "Téléchargement de Hollow $VERSION pour linux/$ARCH..."
if command -v curl >/dev/null 2>&1; then
    curl -fL --silent --show-error -o "$TEMP_FILE" "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
    wget -q -O "$TEMP_FILE" "$DOWNLOAD_URL"
else
    echo "Installez curl ou wget avec : apk add --no-cache curl" >&2
    exit 1
fi

chmod 755 "$TEMP_FILE"

if [ -w "$INSTALL_DIR" ]; then
    install -m 755 "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
elif command -v doas >/dev/null 2>&1; then
    doas install -m 755 "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
elif command -v sudo >/dev/null 2>&1; then
    sudo install -m 755 "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
else
    echo "Permission insuffisante pour écrire dans $INSTALL_DIR." >&2
    echo "Relancez le script en root ou définissez INSTALL_DIR=$HOME/.local/bin." >&2
    exit 1
fi

echo "Hollow $VERSION installé dans $INSTALL_DIR/$BINARY_NAME"

PROFILE_D="/etc/profile.d"
if [ -d "$PROFILE_D" ] && [ -w "$PROFILE_D" ]; then
    cat << EOF > "$PROFILE_D/hollow.sh"
# --- Hollow Shell Integration ---
hollow() {
    $INSTALL_DIR/$BINARY_NAME "\$@"
    local tmp_file="/tmp/hollow_cwd_\$USER"
    if [ -f "\$tmp_file" ]; then
        cd "\$(cat "\$tmp_file")" && rm "\$tmp_file"
    fi
}

alias nano='hollow'
alias pico='hollow'
alias mcedit='hollow'
alias mc='hollow'
export EDITOR='hollow'
export VISUAL='hollow'
EOF
    chmod 644 "$PROFILE_D/hollow.sh"
    echo "Intégration shell et alias configurés dans $PROFILE_D/hollow.sh"
fi