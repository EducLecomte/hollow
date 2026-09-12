#!/bin/bash

# Arrêter le script en cas d'erreur
set -e

    GITHUB_REPO="EducLecomte/hollow" # Remplacez par votre nom d'utilisateur/repo si différent
BINARY_NAME="hollow"
INSTALL_DIR="/usr/local/bin"

echo "--- Installation de Hollow ---"

# Vérification de 'curl' (pour récupérer la dernière version)
if ! command -v curl &> /dev/null; then
    echo "Erreur : 'curl' n'est pas installé. Veuillez l'installer (ex: sudo apt install curl) ou spécifier manuellement la version à télécharger."
    echo "Exemple : ./install.sh v1.0.0"
    exit 1
fi

# Vérification de 'wget' (pour télécharger le binaire)
if ! command -v wget &> /dev/null; then
    echo "Erreur : 'wget' n'est pas installé. Veuillez l'installer (ex: sudo apt install wget)."
    exit 1
fi

# Détection de l'OS et de l'architecture
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

case "$ARCH" in
    x86_64) ARCH="amd64" ;;
    aarch64) ARCH="arm64" ;;
    *) echo "Architecture non supportée par les binaires pré-compilés : $ARCH. Veuillez compiler depuis les sources si vous le souhaitez."; exit 1 ;;
esac

# Récupération de la dernière version ou utilisation de l'argument fourni
VERSION=${1:-}
if [ -z "$VERSION" ]; then
    echo "Recherche de la dernière version sur GitHub..."
    LATEST_VERSION=$(curl -sL "https://api.github.com/repos/${GITHUB_REPO}/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    if [ -z "$LATEST_VERSION" ]; then
        echo "Erreur : Impossible de récupérer la dernière version. Veuillez spécifier une version manuellement (ex: ./install.sh v1.0.0)."
        exit 1
    fi
    VERSION="$LATEST_VERSION"
    echo "Dernière version trouvée : $VERSION"
else
    echo "Utilisation de la version spécifiée : $VERSION"
fi

# Construction de l'URL de téléchargement
DOWNLOAD_URL="https://github.com/${GITHUB_REPO}/releases/download/${VERSION}/${BINARY_NAME}-${OS}-${ARCH}"
TEMP_BINARY="/tmp/${BINARY_NAME}_${VERSION}"

echo "Téléchargement de Hollow (${VERSION}) pour ${OS}/${ARCH} depuis ${DOWNLOAD_URL}..."
wget -q --show-progress -O "$TEMP_BINARY" "$DOWNLOAD_URL"

if [ $? -ne 0 ]; then
    echo "Erreur : Échec du téléchargement du binaire. Vérifiez la version et l'URL."
    rm -f "$TEMP_BINARY"
    exit 1
fi

chmod +x "$TEMP_BINARY"

echo "Installation de Hollow dans $INSTALL_DIR..."
mv "$TEMP_BINARY" "$INSTALL_DIR/$BINARY_NAME"

if [ $? -ne 0 ]; then
    echo "Erreur : Échec de l'installation. Assurez-vous d'avoir les permissions sudo."
    rm -f "$TEMP_BINARY"
    exit 1
fi

echo "Installation de Hollow terminée."

# Détection de l'utilisateur réel pour injecter la configuration au bon endroit
ACTUAL_USER=${SUDO_USER:-$USER}
ACTUAL_HOME=$(getent passwd "$ACTUAL_USER" | cut -d: -f6 || echo "$HOME")

SHELL_FUNC="
# --- Hollow Shell Integration ---
function hollow() {
    local hollow_bin=\"$INSTALL_DIR/$BINARY_NAME\"
    \$hollow_bin \"\$@\"
    local tmp_file=\"/tmp/hollow_cwd_\$USER\"
    if [ -f \"\$tmp_file\" ]; then
        cd \"\$(cat \"\$tmp_file\")\" && rm \"\$tmp_file\"
    fi
}
"

echo "Configuration du shell pour l'utilisateur : $ACTUAL_USER"

for RC_FILE in "$ACTUAL_HOME/.bashrc" "$ACTUAL_HOME/.zshrc"; do
    if [ -f "$RC_FILE" ]; then
        if ! grep -q "function hollow()" "$RC_FILE"; then
            echo "Ajout de la fonction hollow dans $RC_FILE..."
            echo "$SHELL_FUNC" >> "$RC_FILE"
        else
            echo "La fonction hollow est déjà présente dans $RC_FILE."
        fi
    fi
done

echo "Installation réussie ! Relancez votre terminal ou tapez : source $ACTUAL_HOME/.bashrc"
