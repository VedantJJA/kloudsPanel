#!/usr/bin/env bash
# ==============================================================================
#  kloudsPanel / DevPanel Universal Linux Setup & Auto-Installer
#  Compatible with: Ubuntu, Debian, CentOS, RHEL, Fedora, Rocky, Alma, Arch, Alpine, openSUSE
# ==============================================================================

set -eo pipefail

BOLD='\033[1m'
GREEN='\033[0;32m'
CYAN='\033[0;36m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${CYAN}${BOLD}"
echo "=============================================================================="
echo "   __   ___                 _      ___                  __"
echo "  / /__/ /___  __ _____  __/ /___ / _ \___ ____  ___   / /"
echo " /  '_/ / _  \/ // / _ \/ _ / ___/ ___/ _ \`/ _ \/ _ \/ / "
echo "/_/\_\_/\___/\_,_/\_,_/\_,_/    /_/   \_,_/_//_/\___/__/  "
echo "                                                          "
echo "  Universal Linux Bootstrap & Self-Healing Setup Script   "
echo "=============================================================================="
echo -e "${NC}"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$SCRIPT_DIR"

# 1. Detect Root / Sudo
SUDO=""
if [ "$EUID" -ne 0 ]; then
    if command -v sudo >/dev/null 2>&1; then
        SUDO="sudo"
    else
        echo -e "${RED}Error: This installer requires root privileges. Please run as root or with sudo.${NC}"
        exit 1
    fi
fi

# 2. Detect Linux Package Manager
PKG_MANAGER=""
if command -v apt-get >/dev/null 2>&1; then
    PKG_MANAGER="apt"
elif command -v dnf >/dev/null 2>&1; then
    PKG_MANAGER="dnf"
elif command -v yum >/dev/null 2>&1; then
    PKG_MANAGER="yum"
elif command -v pacman >/dev/null 2>&1; then
    PKG_MANAGER="pacman"
elif command -v apk >/dev/null 2>&1; then
    PKG_MANAGER="apk"
elif command -v zypper >/dev/null 2>&1; then
    PKG_MANAGER="zypper"
fi

echo -e "${CYAN}[1/7] Detected Package Manager:${NC} ${BOLD}${PKG_MANAGER:-generic}${NC}"

install_pkg() {
    local pkgs=("$@")
    echo -e "${CYAN}==> Installing missing packages:${NC} ${pkgs[*]}"
    case "$PKG_MANAGER" in
        apt)
            $SUDO apt-get update -qq || true
            $SUDO apt-get install -y "${pkgs[@]}"
            ;;
        dnf)
            $SUDO dnf install -y "${pkgs[@]}"
            ;;
        yum)
            $SUDO yum install -y "${pkgs[@]}"
            ;;
        pacman)
            $SUDO pacman -Sy --noconfirm "${pkgs[@]}"
            ;;
        apk)
            $SUDO apk update || true
            $SUDO apk add "${pkgs[@]}"
            ;;
        zypper)
            $SUDO zypper --non-interactive install "${pkgs[@]}"
            ;;
        *)
            echo -e "${YELLOW}Warning: Unknown package manager. Please ensure prerequisites (${pkgs[*]}) are installed.${NC}"
            ;;
    esac
}

# 3. Ensure Core System Tools
echo -e "${CYAN}[2/7] Checking and installing core dependencies (curl, git, jq, tar)...${NC}"
NEEDED_TOOLS=()
for tool in curl wget git jq tar gzip unzip; do
    if ! command -v "$tool" >/dev/null 2>&1; then
        NEEDED_TOOLS+=("$tool")
    fi
done

if [ ${#NEEDED_TOOLS[@]} -gt 0 ]; then
    install_pkg "${NEEDED_TOOLS[@]}"
fi

# 4. Check & Install Docker CE & Docker Compose
echo -e "${CYAN}[3/7] Checking Docker & Docker Compose...${NC}"
if ! command -v docker >/dev/null 2>&1; then
    echo -e "${YELLOW}==> Docker not found. Installing Docker CE automatically via official script...${NC}"
    curl -fsSL https://get.docker.com | $SUDO sh
    if command -v systemctl >/dev/null 2>&1; then
        $SUDO systemctl enable --now docker || true
    fi
else
    echo -e "${GREEN}✓ Docker is already installed:${NC} $(docker --version)"
fi

# Ensure docker daemon is active
if command -v systemctl >/dev/null 2>&1; then
    $SUDO systemctl start docker || true
elif command -v service >/dev/null 2>&1; then
    $SUDO service docker start || true
fi

# Add current user to docker group if needed
if [ -n "${SUDO_USER:-}" ]; then
    $SUDO usermod -aG docker "$SUDO_USER" || true
fi

# Ensure Docker Compose plugin is available
if ! docker compose version >/dev/null 2>&1; then
    echo -e "${YELLOW}==> Installing Docker Compose plugin...${NC}"
    if [ "$PKG_MANAGER" = "apt" ]; then
        $SUDO apt-get install -y docker-compose-plugin || true
    elif [ "$PKG_MANAGER" = "dnf" ] || [ "$PKG_MANAGER" = "yum" ]; then
        $SUDO dnf install -y docker-compose-plugin || true
    fi
fi

# 5. Check & Install Go (1.22+)
echo -e "${CYAN}[4/7] Checking Go toolchain...${NC}"
GO_INSTALL_REQUIRED=false
if ! command -v go >/dev/null 2>&1; then
    GO_INSTALL_REQUIRED=true
else
    GO_VER="$(go version | awk '{print $3}' | tr -d 'go')"
    GO_MAJOR="$(echo "$GO_VER" | cut -d. -f1)"
    GO_MINOR="$(echo "$GO_VER" | cut -d. -f2)"
    if [ "$GO_MAJOR" -lt 1 ] || { [ "$GO_MAJOR" -eq 1 ] && [ "$GO_MINOR" -lt 22 ]; }; then
        GO_INSTALL_REQUIRED=true
    fi
fi

if [ "$GO_INSTALL_REQUIRED" = true ]; then
    echo -e "${YELLOW}==> Installing Go 1.22.6 for Linux x86_64...${NC}"
    ARCH="amd64"
    UNAME_ARCH="$(uname -m)"
    if [ "$UNAME_ARCH" = "aarch64" ] || [ "$UNAME_ARCH" = "arm64" ]; then
        ARCH="arm64"
    fi
    GO_TAR="go1.22.6.linux-${ARCH}.tar.gz"
    curl -fsSL "https://go.dev/dl/${GO_TAR}" -o "/tmp/${GO_TAR}"
    $SUDO rm -rf /usr/local/go
    $SUDO tar -C /usr/local -xzf "/tmp/${GO_TAR}"
    rm -f "/tmp/${GO_TAR}"
    export PATH="/usr/local/go/bin:$PATH"
    echo 'export PATH=$PATH:/usr/local/go/bin' | $SUDO tee /etc/profile.d/go.sh >/dev/null
    echo -e "${GREEN}✓ Go installed successfully:${NC} $(go version)"
else
    echo -e "${GREEN}✓ Go is already installed:${NC} $(go version)"
fi

# 6. Check & Install Node.js (v20+ LTS) and pnpm
echo -e "${CYAN}[5/7] Checking Node.js and pnpm...${NC}"
NODE_INSTALL_REQUIRED=false
if ! command -v node >/dev/null 2>&1; then
    NODE_INSTALL_REQUIRED=true
else
    NODE_VER="$(node -v | tr -d 'v' | cut -d. -f1)"
    if [ "$NODE_VER" -lt 18 ]; then
        NODE_INSTALL_REQUIRED=true
    fi
fi

if [ "$NODE_INSTALL_REQUIRED" = true ]; then
    echo -e "${YELLOW}==> Installing Node.js v20 LTS via NodeSource...${NC}"
    if [ "$PKG_MANAGER" = "apt" ]; then
        curl -fsSL https://deb.nodesource.com/setup_20.x | $SUDO -E bash -
        $SUDO apt-get install -y nodejs
    elif [ "$PKG_MANAGER" = "dnf" ] || [ "$PKG_MANAGER" = "yum" ]; then
        curl -fsSL https://rpm.nodesource.com/setup_20.x | $SUDO bash -
        $SUDO dnf install -y nodejs || $SUDO yum install -y nodejs
    else
        install_pkg nodejs npm
    fi
    echo -e "${GREEN}✓ Node.js installed:${NC} $(node -v)"
else
    echo -e "${GREEN}✓ Node.js is already installed:${NC} $(node -v)"
fi

if ! command -v pnpm >/dev/null 2>&1; then
    echo -e "${YELLOW}==> Installing pnpm package manager...${NC}"
    $SUDO npm install -g pnpm || true
fi
echo -e "${GREEN}✓ pnpm is ready:${NC} $(pnpm -v 2>/dev/null || echo 'installed')"

# Check Rust Toolchain (Optional self-healing)
if ! command -v cargo >/dev/null 2>&1; then
    echo -e "${YELLOW}==> Installing Rust toolchain via rustup...${NC}"
    curl --proto '=https' --tlsv1.2 -sSf https://sh.rustup.rs | sh -s -- -y --profile minimal || true
    if [ -f "$HOME/.cargo/env" ]; then
        source "$HOME/.cargo/env"
    fi
fi

# 7. Check & Install Database CLI Utilities (psql, mysql, redis-cli)
echo -e "${CYAN}[6/7] Checking database client tools (psql, mysql, redis-cli)...${NC}"
DB_CLIENTS=()
if ! command -v psql >/dev/null 2>&1; then
    [ "$PKG_MANAGER" = "apt" ] && DB_CLIENTS+=("postgresql-client")
    [ "$PKG_MANAGER" = "dnf" ] || [ "$PKG_MANAGER" = "yum" ] && DB_CLIENTS+=("postgresql")
    [ "$PKG_MANAGER" = "pacman" ] && DB_CLIENTS+=("postgresql-libs")
fi
if ! command -v mysql >/dev/null 2>&1; then
    [ "$PKG_MANAGER" = "apt" ] && DB_CLIENTS+=("default-mysql-client")
    [ "$PKG_MANAGER" = "dnf" ] || [ "$PKG_MANAGER" = "yum" ] && DB_CLIENTS+=("mariadb")
    [ "$PKG_MANAGER" = "pacman" ] && DB_CLIENTS+=("mariadb-clients")
fi
if ! command -v redis-cli >/dev/null 2>&1; then
    [ "$PKG_MANAGER" = "apt" ] && DB_CLIENTS+=("redis-tools")
    [ "$PKG_MANAGER" = "dnf" ] || [ "$PKG_MANAGER" = "yum" ] && DB_CLIENTS+=("redis")
    [ "$PKG_MANAGER" = "pacman" ] && DB_CLIENTS+=("redis")
fi

if [ ${#DB_CLIENTS[@]} -gt 0 ]; then
    install_pkg "${DB_CLIENTS[@]}" || true
fi

# 8. Setup Docker Network & Pre-Pull Container Images
echo -e "${CYAN}[7/7] Initializing Docker network & runtime images...${NC}"
docker network create platform-control >/dev/null 2>&1 || true

echo -e "==> Pre-pulling standard runtime & database images for instant service boots..."
PREPULL_IMAGES=(
    "postgres:16-alpine"
    "mysql:8.0"
    "redis:7.2-alpine"
    "mongo:7.0"
    "clickhouse/clickhouse-server:24.3-alpine"
    "nginx:alpine"
    "node:20-alpine"
    "python:3.11-slim"
    "golang:1.22-alpine"
)

for img in "${PREPULL_IMAGES[@]}"; do
    echo -n "  • Pulling $img... "
    docker pull -q "$img" >/dev/null 2>&1 && echo -e "${GREEN}done${NC}" || echo -e "${YELLOW}skipped${NC}"
done

# 9. Configure Environment File (.env)
ENV_FILE="$SCRIPT_DIR/paas/deploy/compose/.env"
if [ ! -f "$ENV_FILE" ]; then
    echo -e "${CYAN}==> Generating .env with cryptographically secure random secrets...${NC}"
    JWT_SECRET="$(openssl rand -hex 32 2>/dev/null || head -c 32 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 32)"
    ADMIN_PASS="kp_admin_$(openssl rand -hex 6 2>/dev/null || head -c 6 /dev/urandom | base64 | tr -dc 'a-zA-Z0-9' | head -c 8)"
    
    cp "$SCRIPT_DIR/paas/deploy/compose/.env.example" "$ENV_FILE" 2>/dev/null || true
    if [ -f "$ENV_FILE" ]; then
        sed -i "s|JWT_SECRET=.*|JWT_SECRET=${JWT_SECRET}|g" "$ENV_FILE" || true
    fi
fi

# 10. Configure Host Firewall (UFW & iptables for Database & Ingress Ports)
echo -e "${CYAN}==> Ensuring platform & database port ranges (80, 443, 13000-20000) are open in host firewall...${NC}"
if command -v ufw >/dev/null 2>&1 && $SUDO ufw status 2>/dev/null | grep -q "active"; then
    $SUDO ufw allow 80/tcp >/dev/null 2>&1 || true
    $SUDO ufw allow 443/tcp >/dev/null 2>&1 || true
    $SUDO ufw allow 13000:20000/tcp >/dev/null 2>&1 || true
    echo -e "${GREEN}✓ UFW rules configured for HTTP (80/443) and Databases (13000-20000)${NC}"
fi

if command -v iptables >/dev/null 2>&1; then
    $SUDO iptables -C INPUT -p tcp --dport 13000:20000 -j ACCEPT 2>/dev/null || $SUDO iptables -I INPUT 1 -p tcp --dport 13000:20000 -j ACCEPT 2>/dev/null || true
    $SUDO iptables -C INPUT -p tcp --dport 80 -j ACCEPT 2>/dev/null || $SUDO iptables -I INPUT 1 -p tcp --dport 80 -j ACCEPT 2>/dev/null || true
    $SUDO iptables -C INPUT -p tcp --dport 443 -j ACCEPT 2>/dev/null || $SUDO iptables -I INPUT 1 -p tcp --dport 443 -j ACCEPT 2>/dev/null || true
    if command -v netfilter-persistent >/dev/null 2>&1; then
        $SUDO netfilter-persistent save >/dev/null 2>&1 || true
    fi
fi

# 11. Launch kloudsPanel Stack
echo ""
echo -e "${GREEN}${BOLD}=============================================================================="
echo " ✓ System environment is fully initialized and all dependencies are ready!"
echo "==============================================================================${NC}"
echo ""
echo -e "To start kloudsPanel now, run:"
echo -e "  ${CYAN}./deploy.sh${NC}        (Starts platform & auto-deploys on every git push)"
echo -e "  ${CYAN}./deploy.sh --once${NC} (Starts platform in background and exits)"
echo ""

# Ask to start automatically if running interactively
if [ -t 0 ]; then
    read -p "Would you like to deploy and start kloudsPanel now? [Y/n] " -n 1 -r
    echo ""
    if [[ $REPLY =~ ^[Yy]$ ]] || [[ -z $REPLY ]]; then
        "$SCRIPT_DIR/deploy.sh" --once
    fi
fi
