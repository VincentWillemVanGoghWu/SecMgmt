#!/usr/bin/env bash

set -Eeuo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_NAME="secmgmt_go"
SERVICE_NAME="secmgmt-go"
COMPOSE_PROJECT_NAME="secmgmt-go"
COMPOSE_FILE="${SCRIPT_DIR}/docker-compose.yml"
INSTALL_DIR="${INSTALL_DIR:-/opt/secmgmt_go}"
RUNTIME_DIR="${INSTALL_DIR}/runtime"
CACHE_DIR="${INSTALL_DIR}/.deploy-cache"
GENERATED_DIR="${RUNTIME_DIR}/generated"
MYSQL_DATA_DIR="${RUNTIME_DIR}/mysql"
MEDIA_DIR="${RUNTIME_DIR}/media"
BACKEND_ENV_FILE="${GENERATED_DIR}/backend.env"
COMPOSE_ENV_FILE="${GENERATED_DIR}/compose.env"
NGINX_CONF_FILE="${GENERATED_DIR}/nginx.default.conf"
BACKEND_PORT="${BACKEND_PORT:-8000}"
FRONTEND_PORT="${FRONTEND_PORT:-80}"
SERVER_NAME="${SERVER_NAME:-_}"
PUBLIC_BASE_URL="${PUBLIC_BASE_URL:-}"
MYSQL_APP_DB="${MYSQL_APP_DB:-secmgmt_db}"
MYSQL_APP_USER="${MYSQL_APP_USER:-secmgmt}"
MYSQL_APP_PASSWORD="${MYSQL_APP_PASSWORD:-}"
MYSQL_ROOT_PASSWORD="${MYSQL_ROOT_PASSWORD:-}"
SQL_DIR_NAME="sql"
FORCE_INSTALL=0
FORCE_REBUILD=0
REBUILD_TARGETS=""
REINIT_DB=0
DEFAULT_APT_MIRROR_URL="https://mirrors.aliyun.com/ubuntu"
DEFAULT_APT_SECURITY_MIRROR_URL="https://mirrors.aliyun.com/ubuntu"
DEFAULT_DEBIAN_APT_MIRROR_URL="http://mirrors.aliyun.com/debian"
DEFAULT_DEBIAN_APT_SECURITY_MIRROR_URL="http://mirrors.aliyun.com/debian-security"
DEFAULT_GOPROXY="https://goproxy.cn,direct"
DEFAULT_NPM_REGISTRY="https://registry.npmmirror.com"
DEFAULT_DOCKER_LIBRARY_MIRROR="docker.m.daocloud.io/library"
APT_MIRROR_URL="${APT_MIRROR_URL:-${DEFAULT_APT_MIRROR_URL}}"
APT_SECURITY_MIRROR_URL="${APT_SECURITY_MIRROR_URL:-${DEFAULT_APT_SECURITY_MIRROR_URL}}"
DEBIAN_APT_MIRROR_URL="${DEBIAN_APT_MIRROR_URL:-${DEFAULT_DEBIAN_APT_MIRROR_URL}}"
DEBIAN_APT_SECURITY_MIRROR_URL="${DEBIAN_APT_SECURITY_MIRROR_URL:-${DEFAULT_DEBIAN_APT_SECURITY_MIRROR_URL}}"
GOPROXY="${GOPROXY:-${DEFAULT_GOPROXY}}"
NPM_REGISTRY="${NPM_REGISTRY:-${DEFAULT_NPM_REGISTRY}}"
DOCKER_LIBRARY_MIRROR="${DOCKER_LIBRARY_MIRROR:-${DEFAULT_DOCKER_LIBRARY_MIRROR}}"

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

log_info() {
  echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $*" >&2
}

usage() {
  cat <<'EOF'
Usage:
  sudo ./deploy_ubuntu.sh [options]

Options:
  --install-dir <path>         Runtime data dir, default: /opt/secmgmt_go
  --backend-port <port>        Host port mapped to backend, default: 8000
  --frontend-port <port>       Host port mapped to frontend nginx, default: 80
  --server-name <name>         Nginx server_name, default: _
  --public-base-url <url>      Public base URL, e.g. http://10.0.0.8 or https://demo.example.com
  --mysql-db <name>            MySQL app database name, default: secmgmt_db
  --mysql-user <name>          MySQL app username, default: secmgmt
  --mysql-password <pass>      MySQL app password, default: auto-generate or reuse existing
  --mysql-root-password <pass> MySQL root password, default: auto-generate or reuse existing
  --reinit-db                  Reserved flag, database import already rebuilds by default
  --force-install              Force apt-get update/install even if packages already exist
  --force-rebuild              Force backend and frontend image rebuild without cache
  --rebuild <target>           Force selected image rebuild without cache: backend, frontend, all
                               Multiple targets can be comma-separated, e.g. backend,frontend
  --rebuild-backend            Force backend image rebuild without cache
  --rebuild-frontend           Force frontend image rebuild without cache
  --apt-mirror-url <url>       Ubuntu apt mirror, default: https://mirrors.aliyun.com/ubuntu
  --apt-security-mirror <url>  Ubuntu security mirror, default: https://mirrors.aliyun.com/ubuntu
  --goproxy <url>              Go module proxy, default: https://goproxy.cn,direct
  --npm-registry <url>         npm registry, default: https://registry.npmmirror.com
  --docker-library-mirror <v>  Base image mirror prefix, default: docker.m.daocloud.io/library
  -h, --help                   Show help

Examples:
  sudo ./deploy_ubuntu.sh
  sudo ./deploy_ubuntu.sh --frontend-port 8080 --backend-port 18000
  sudo ./deploy_ubuntu.sh --server-name demo.example.com --public-base-url https://demo.example.com
  sudo ./deploy_ubuntu.sh --mysql-db secmgmt_prod --mysql-user secmgmt --mysql-password strong-password
  sudo ./deploy_ubuntu.sh --force-rebuild
  sudo ./deploy_ubuntu.sh --rebuild backend
  sudo ./deploy_ubuntu.sh --rebuild frontend
EOF
}

append_rebuild_target() {
  local raw="$1"
  local target
  raw="${raw// /}"
  if [[ -z "${raw}" ]]; then
    log_error "Rebuild target cannot be empty."
    exit 1
  fi

  IFS=',' read -ra targets <<< "${raw}"
  for target in "${targets[@]}"; do
    case "${target}" in
      all)
        append_rebuild_target "backend,frontend"
        ;;
      backend|frontend)
        if [[ ",${REBUILD_TARGETS}," != *",${target},"* ]]; then
          if [[ -z "${REBUILD_TARGETS}" ]]; then
            REBUILD_TARGETS="${target}"
          else
            REBUILD_TARGETS="${REBUILD_TARGETS},${target}"
          fi
        fi
        ;;
      *)
        log_error "Unsupported rebuild target: ${target}"
        log_error "Supported targets: backend, frontend, all"
        exit 1
        ;;
    esac
  done
}

parse_args() {
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --install-dir)
        INSTALL_DIR="$2"
        shift 2
        ;;
      --backend-port)
        BACKEND_PORT="$2"
        shift 2
        ;;
      --frontend-port)
        FRONTEND_PORT="$2"
        shift 2
        ;;
      --server-name)
        SERVER_NAME="$2"
        shift 2
        ;;
      --public-base-url)
        PUBLIC_BASE_URL="$2"
        shift 2
        ;;
      --mysql-db)
        MYSQL_APP_DB="$2"
        shift 2
        ;;
      --mysql-user)
        MYSQL_APP_USER="$2"
        shift 2
        ;;
      --mysql-password)
        MYSQL_APP_PASSWORD="$2"
        shift 2
        ;;
      --mysql-root-password)
        MYSQL_ROOT_PASSWORD="$2"
        shift 2
        ;;
      --apt-mirror-url)
        APT_MIRROR_URL="$2"
        shift 2
        ;;
      --apt-security-mirror)
        APT_SECURITY_MIRROR_URL="$2"
        shift 2
        ;;
      --goproxy)
        GOPROXY="$2"
        shift 2
        ;;
      --npm-registry)
        NPM_REGISTRY="$2"
        shift 2
        ;;
      --docker-library-mirror)
        DOCKER_LIBRARY_MIRROR="$2"
        shift 2
        ;;
      --reinit-db)
        REINIT_DB=1
        shift
        ;;
      --force-install)
        FORCE_INSTALL=1
        shift
        ;;
      --force-rebuild)
        FORCE_REBUILD=1
        append_rebuild_target "backend,frontend"
        shift
        ;;
      --rebuild)
        if [[ $# -lt 2 ]]; then
          log_error "--rebuild requires a target: backend, frontend, or all"
          exit 1
        fi
        append_rebuild_target "$2"
        shift 2
        ;;
      --rebuild-backend)
        append_rebuild_target "backend"
        shift
        ;;
      --rebuild-frontend)
        append_rebuild_target "frontend"
        shift
        ;;
      -h|--help)
        usage
        exit 0
        ;;
      *)
        log_error "Unsupported argument: $1"
        usage
        exit 1
        ;;
    esac
  done

  RUNTIME_DIR="${INSTALL_DIR}/runtime"
  CACHE_DIR="${INSTALL_DIR}/.deploy-cache"
  GENERATED_DIR="${RUNTIME_DIR}/generated"
  MYSQL_DATA_DIR="${RUNTIME_DIR}/mysql"
  MEDIA_DIR="${RUNTIME_DIR}/media"
  BACKEND_ENV_FILE="${GENERATED_DIR}/backend.env"
  COMPOSE_ENV_FILE="${GENERATED_DIR}/compose.env"
  NGINX_CONF_FILE="${GENERATED_DIR}/nginx.default.conf"

  if ! [[ "${BACKEND_PORT}" =~ ^[0-9]{1,5}$ ]]; then
    log_error "Invalid backend port: ${BACKEND_PORT}"
    exit 1
  fi
  if ! [[ "${FRONTEND_PORT}" =~ ^[0-9]{1,5}$ ]]; then
    log_error "Invalid frontend port: ${FRONTEND_PORT}"
    exit 1
  fi
  if ! [[ "${MYSQL_APP_DB}" =~ ^[A-Za-z0-9_]+$ ]]; then
    log_error "MySQL database name only supports letters, digits and underscore."
    exit 1
  fi
  if ! [[ "${MYSQL_APP_USER}" =~ ^[A-Za-z0-9_]+$ ]]; then
    log_error "MySQL username only supports letters, digits and underscore."
    exit 1
  fi
}

require_root() {
  if [[ "$(id -u)" -ne 0 ]]; then
    log_error "Please run with sudo or as root."
    exit 1
  fi
}

check_os() {
  if [[ ! -f /etc/os-release ]]; then
    log_error "Cannot detect operating system."
    exit 1
  fi

  # shellcheck disable=SC1091
  source /etc/os-release
  if [[ "${ID:-}" != "ubuntu" ]]; then
    log_error "This script only supports Ubuntu. Current system: ${PRETTY_NAME:-unknown}"
    exit 1
  fi

  if [[ "$(dpkg --print-architecture)" != "amd64" ]]; then
    log_error "This script only supports amd64."
    exit 1
  fi
}

ensure_cache_dir() {
  mkdir -p "${CACHE_DIR}"
}

configure_apt_mirror() {
  local changed=0
  local file=""
  local temp_file=""
  local files=()

  if [[ -f /etc/apt/sources.list ]]; then
    files+=("/etc/apt/sources.list")
  fi

  while IFS= read -r -d '' file; do
    files+=("${file}")
  done < <(find /etc/apt/sources.list.d -maxdepth 1 -type f \( -name '*.list' -o -name '*.sources' \) -print0 2>/dev/null)

  for file in "${files[@]}"; do
    temp_file="$(mktemp)"
    sed \
      -e "s|https\?://\(archive\|ports\)\.ubuntu\.com/ubuntu/?|${APT_MIRROR_URL}/|g" \
      -e "s|https\?://security\.ubuntu\.com/ubuntu/?|${APT_SECURITY_MIRROR_URL}/|g" \
      -e "s|https\?://cn\.archive\.ubuntu\.com/ubuntu/?|${APT_MIRROR_URL}/|g" \
      "${file}" > "${temp_file}"
    if ! cmp -s "${file}" "${temp_file}"; then
      cp "${temp_file}" "${file}"
      changed=1
    fi
    rm -f "${temp_file}"
  done

  if [[ "${changed}" -eq 1 ]]; then
    log_info "Ubuntu apt mirror updated to domestic sources."
  else
    log_info "Ubuntu apt mirror already points to configured sources, skipping."
  fi
}

is_package_installed() {
  local pkg="$1"
  dpkg-query -W -f='${Status}' "${pkg}" 2>/dev/null | grep -q '^install ok installed$'
}

apt_metadata_is_fresh() {
  if find /var/lib/apt/lists -maxdepth 1 -type f -name '*_Packages' -mmin -360 2>/dev/null | grep -q .; then
    return 0
  fi
  if [[ -f /var/lib/apt/periodic/update-success-stamp ]] && find /var/lib/apt/periodic/update-success-stamp -mmin -360 2>/dev/null | grep -q .; then
    return 0
  fi
  return 1
}

configure_docker_daemon() {
  mkdir -p /etc/docker

  local tmp_daemon
  tmp_daemon="$(mktemp)"

  cat > "${tmp_daemon}" <<'EOF'
{
  "registry-mirrors": [
    "https://docker.1ms.run",
    "https://docker.m.daocloud.io",
    "https://hub-mirror.c.163.com"
  ],
  "exec-opts": [
    "native.cgroupdriver=systemd"
  ],
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "3"
  },
  "live-restore": true
}
EOF

  if [[ ! -f /etc/docker/daemon.json ]] || ! cmp -s "${tmp_daemon}" /etc/docker/daemon.json; then
    cp "${tmp_daemon}" /etc/docker/daemon.json
    DOCKER_DAEMON_CHANGED=1
    log_info "Docker registry mirrors updated to domestic accelerators."
  else
    DOCKER_DAEMON_CHANGED=0
    log_info "Docker daemon mirror configuration already up to date."
  fi

  rm -f "${tmp_daemon}"
}

ensure_apt_metadata() {
  if [[ "${FORCE_INSTALL}" -eq 1 ]]; then
    log_info "Force install enabled, running apt-get update."
    apt-get update -y
    return
  fi
  if apt_metadata_is_fresh; then
    log_info "APT metadata is fresh, skipping apt-get update."
    return
  fi
  apt-get update -y
}

hash_paths() {
  local path=""
  local file=""

  for path in "$@"; do
    if [[ -f "${path}" ]]; then
      sha256sum "${path}"
    elif [[ -d "${path}" ]]; then
      while IFS= read -r -d '' file; do
        sha256sum "${file}"
      done < <(find "${path}" -type f -print0 | sort -z)
    fi
  done | sha256sum | awk '{print $1}'
}

read_stamp() {
  local stamp_file="$1"
  if [[ -f "${stamp_file}" ]]; then
    cat "${stamp_file}"
  fi
}

write_stamp() {
  local stamp_file="$1"
  local value="$2"
  mkdir -p "$(dirname "${stamp_file}")"
  printf '%s' "${value}" > "${stamp_file}"
}

install_packages() {
  export DEBIAN_FRONTEND=noninteractive
  local packages=(
    ca-certificates
    curl
  )
  local missing_packages=()
  local pkg=""

  if ! command -v docker >/dev/null 2>&1; then
    packages+=(docker.io)
  fi
  if ! docker compose version >/dev/null 2>&1 && ! command -v docker-compose >/dev/null 2>&1; then
    packages+=(docker-compose)
  fi

  for pkg in "${packages[@]}"; do
    if ! is_package_installed "${pkg}"; then
      missing_packages+=("${pkg}")
    fi
  done

  configure_apt_mirror

  if [[ "${FORCE_INSTALL}" -eq 0 && "${#missing_packages[@]}" -eq 0 ]]; then
    log_info "Required Ubuntu packages already installed, skipping apt-get install."
    return
  fi

  ensure_apt_metadata
  if [[ "${FORCE_INSTALL}" -eq 1 ]]; then
    apt-get install -y "${packages[@]}"
  else
    apt-get install -y "${missing_packages[@]}"
  fi
}

generate_secret() {
  openssl rand -hex 24
}

read_env_key() {
  local file="$1"
  local key="$2"
  if [[ -f "${file}" ]]; then
    sed -n "s/^${key}=//p" "${file}" | head -n 1
  fi
}

ensure_mysql_passwords() {
  if [[ -z "${MYSQL_APP_PASSWORD}" ]]; then
    MYSQL_APP_PASSWORD="$(read_env_key "${COMPOSE_ENV_FILE}" "MYSQL_APP_PASSWORD")"
  fi
  if [[ -z "${MYSQL_ROOT_PASSWORD}" ]]; then
    MYSQL_ROOT_PASSWORD="$(read_env_key "${COMPOSE_ENV_FILE}" "MYSQL_ROOT_PASSWORD")"
  fi
  if [[ -z "${MYSQL_APP_PASSWORD}" ]]; then
    MYSQL_APP_PASSWORD="$(generate_secret)"
  fi
  if [[ -z "${MYSQL_ROOT_PASSWORD}" ]]; then
    MYSQL_ROOT_PASSWORD="$(generate_secret)"
  fi
}

ensure_runtime_dirs() {
  mkdir -p "${GENERATED_DIR}" "${MYSQL_DATA_DIR}" "${MEDIA_DIR}"
}

ensure_docker_running() {
  configure_docker_daemon
  systemctl enable docker
  systemctl restart docker
  if [[ "${DOCKER_DAEMON_CHANGED:-0}" -eq 1 ]]; then
    sleep 2
  fi
  if docker compose version >/dev/null 2>&1; then
    return
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    return
  fi
  log_error "Neither docker compose nor docker-compose is available after installation."
  exit 1
}

compose_command() {
  if docker compose version >/dev/null 2>&1; then
    printf 'docker compose'
    return
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    printf 'docker-compose'
    return
  fi
  log_error "No compose command available."
  exit 1
}

compose() {
  local compose_bin
  compose_bin="$(compose_command)"
  if [[ "${compose_bin}" == "docker compose" ]]; then
    docker compose --project-name "${COMPOSE_PROJECT_NAME}" --env-file "${COMPOSE_ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
    return
  fi
  if [[ "$1" == "exec" ]]; then
    shift
    docker-compose --project-name "${COMPOSE_PROJECT_NAME}" --env-file "${COMPOSE_ENV_FILE}" -f "${COMPOSE_FILE}" exec "$@"
    return
  fi
  docker-compose --project-name "${COMPOSE_PROJECT_NAME}" --env-file "${COMPOSE_ENV_FILE}" -f "${COMPOSE_FILE}" "$@"
}

stop_host_conflicting_services() {
  local svc=""
  for svc in nginx "${SERVICE_NAME}"; do
    if systemctl list-unit-files | grep -q "^${svc}\.service"; then
      log_info "Stopping host service ${svc} to avoid port conflicts with docker compose."
      systemctl stop "${svc}" || true
    fi
  done
}

frontend_port_in_use() {
  ss -H -ltn "sport = :${FRONTEND_PORT}" 2>/dev/null | grep -q .
}

frontend_port_owned_by_compose() {
  if ! command -v docker >/dev/null 2>&1; then
    return 1
  fi

  docker ps --format '{{.Names}} {{.Ports}}' \
    | grep -E "^${COMPOSE_PROJECT_NAME}[-_]frontend[-_]1 " \
    | grep -E "(:${FRONTEND_PORT}->|0\.0\.0\.0:${FRONTEND_PORT}|:::${FRONTEND_PORT})" >/dev/null 2>&1
}

check_frontend_port_available() {
  if ! frontend_port_in_use; then
    return
  fi

  if frontend_port_owned_by_compose; then
    log_info "Host frontend port ${FRONTEND_PORT}/tcp is used by the existing ${COMPOSE_PROJECT_NAME} frontend container; continuing."
    return
  fi

  log_error "Host frontend port ${FRONTEND_PORT}/tcp is already in use."
  log_error "Use another port, for example: sudo ./deploy_ubuntu.sh --frontend-port 8080"
  log_error "Or stop the service/container that is listening on port ${FRONTEND_PORT}."
  if command -v ss >/dev/null 2>&1; then
    ss -ltnp "sport = :${FRONTEND_PORT}" || true
  fi
  if command -v docker >/dev/null 2>&1; then
    docker ps --format 'table {{.Names}}\t{{.Ports}}' | grep -E "(^NAMES|:${FRONTEND_PORT}->|0\.0\.0\.0:${FRONTEND_PORT}|:::${FRONTEND_PORT})" || true
  fi
  exit 1
}

log_deployment_mode() {
  log_info "Deployment mode: docker compose."
  log_warn "Host MySQL/Redis/Nginx services are not used by this script."
  if docker compose version >/dev/null 2>&1; then
    log_info "Compose command: docker compose"
    return
  fi
  if command -v docker-compose >/dev/null 2>&1; then
    log_info "Compose command: docker-compose"
    return
  fi
  log_error "No compose command available after Docker startup."
  exit 1
}

append_port_if_needed() {
  local base_url="$1"
  if [[ "${FRONTEND_PORT}" == "80" ]]; then
    printf '%s' "${base_url}"
  else
    printf '%s:%s' "${base_url}" "${FRONTEND_PORT}"
  fi
}

guess_public_base_url() {
  if [[ -n "${PUBLIC_BASE_URL}" ]]; then
    printf '%s' "${PUBLIC_BASE_URL}"
    return
  fi

  if [[ "${SERVER_NAME}" != "_" ]]; then
    append_port_if_needed "http://${SERVER_NAME}"
    return
  fi

  local detected_ip
  detected_ip="$(hostname -I 2>/dev/null | awk '{print $1}')"
  if [[ -n "${detected_ip}" ]]; then
    append_port_if_needed "http://${detected_ip}"
    return
  fi

  append_port_if_needed "http://127.0.0.1"
}

ensure_env_secret_value() {
  local key="$1"
  local current=""

  current="$(read_env_key "${BACKEND_ENV_FILE}" "${key}")"
  case "${current}" in
    ""|"change-me"|"change-ai-signature-secret")
      generate_secret
      ;;
    *)
      printf '%s' "${current}"
      ;;
  esac
}

write_backend_env() {
  local public_url
  local jwt_secret
  local device_secret
  local ai_secret

  public_url="$(guess_public_base_url)"
  jwt_secret="$(ensure_env_secret_value "JWT_SECRET_KEY")"
  device_secret="$(ensure_env_secret_value "DEVICE_SECRET_KEY")"
  ai_secret="$(ensure_env_secret_value "AI_CALLBACK_SECRET")"

  cat > "${BACKEND_ENV_FILE}" <<EOF
APP_NAME=secmgmt-go
APP_ENV=production
HTTP_PORT=8000
MYSQL_DSN=${MYSQL_APP_USER}:${MYSQL_APP_PASSWORD}@tcp(mysql:3306)/${MYSQL_APP_DB}?charset=utf8mb4&parseTime=True&loc=Local
REDIS_ADDR=127.0.0.1:6379
REDIS_DB=0
JWT_SECRET_KEY=${jwt_secret}
DEVICE_SECRET_KEY=${device_secret}
JWT_EXPIRE_MINUTES=1440
HIKVISION_SDK_PATH=third_party/HCNetSDK_Linux64
MEDIA_ROOT_DIR=/app/media
MEDIA_MOUNT_PATH=/media
BACKEND_PUBLIC_BASE_URL=${public_url}
AI_CALLBACK_SECRET=${ai_secret}
PUSH_HTTP_TIMEOUT_SECONDS=10
EOF
}

write_compose_env() {
  cat > "${COMPOSE_ENV_FILE}" <<EOF
MYSQL_ROOT_PASSWORD=${MYSQL_ROOT_PASSWORD}
MYSQL_APP_DB=${MYSQL_APP_DB}
MYSQL_APP_USER=${MYSQL_APP_USER}
MYSQL_APP_PASSWORD=${MYSQL_APP_PASSWORD}
MYSQL_DATA_DIR=${MYSQL_DATA_DIR}
MEDIA_DIR=${MEDIA_DIR}
BACKEND_ENV_FILE=${BACKEND_ENV_FILE}
NGINX_CONF_FILE=${NGINX_CONF_FILE}
BACKEND_PORT=${BACKEND_PORT}
FRONTEND_PORT=${FRONTEND_PORT}
APT_MIRROR_URL=${APT_MIRROR_URL}
APT_SECURITY_MIRROR_URL=${APT_SECURITY_MIRROR_URL}
DEBIAN_APT_MIRROR_URL=${DEBIAN_APT_MIRROR_URL}
DEBIAN_APT_SECURITY_MIRROR_URL=${DEBIAN_APT_SECURITY_MIRROR_URL}
GOPROXY=${GOPROXY}
NPM_REGISTRY=${NPM_REGISTRY}
DOCKER_LIBRARY_MIRROR=${DOCKER_LIBRARY_MIRROR}
EOF
}

write_nginx_config() {
  cat > "${NGINX_CONF_FILE}" <<EOF
map \$http_upgrade \$connection_upgrade {
    default upgrade;
    '' close;
}

map \$http_x_hik_proxy_target \$hik_http_proxy_target {
    default \$http_x_hik_proxy_target;
    '' \$cookie_webVideoCtrlProxy;
}

map \$arg___hikProxyTarget \$hik_ws_proxy_target {
    default \$arg___hikProxyTarget;
    '' \$cookie_webVideoCtrlProxyWs;
}

map \$hik_ws_proxy_target \$hik_ws_proxy_target_fallback {
    default \$hik_ws_proxy_target;
    '' \$cookie_webVideoCtrlProxyWss;
}

server {
    listen 80;
    server_name ${SERVER_NAME};

    root /usr/share/nginx/html;
    index index.html;

    client_max_body_size 50m;
    add_header Cross-Origin-Embedder-Policy require-corp always;
    add_header Cross-Origin-Opener-Policy same-origin always;
    add_header Cross-Origin-Resource-Policy cross-origin always;

    location /api/ {
        proxy_pass http://backend:8000;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
    }

    location ^~ /media/ {
        proxy_pass http://backend:8000;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        add_header Cache-Control "no-store" always;
    }

    location /healthz {
        proxy_pass http://backend:8000;
        proxy_http_version 1.1;
        proxy_set_header Host \$host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
    }

    location ~ ^/(ISAPI|SDK|PSIA)/ {
        if (\$hik_http_proxy_target = '') {
            return 502 '{"message":"missing hik proxy target"}';
        }

        proxy_pass \$hik_http_proxy_target;
        proxy_http_version 1.1;
        proxy_set_header Host \$proxy_host;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_set_header Origin "";
        proxy_set_header Referer "";
        proxy_set_header Sec-Fetch-Site "";
        proxy_set_header Sec-Fetch-Mode "";
        proxy_set_header Sec-Fetch-Dest "";
        proxy_buffering off;
        proxy_request_buffering off;
        proxy_ssl_server_name on;
    }

    location /webSocketVideoCtrlProxy {
        if (\$hik_ws_proxy_target_fallback = '') {
            return 502 '{"message":"missing hik websocket proxy target"}';
        }

        proxy_pass \$hik_ws_proxy_target_fallback;
        proxy_http_version 1.1;
        proxy_set_header Host \$proxy_host;
        proxy_set_header Upgrade \$http_upgrade;
        proxy_set_header Connection \$connection_upgrade;
        proxy_set_header X-Real-IP \$remote_addr;
        proxy_set_header X-Forwarded-For \$proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto \$scheme;
        proxy_read_timeout 300s;
        proxy_send_timeout 300s;
        proxy_ssl_server_name on;
    }

    location ~* \.(css|js|ico|png|jpg|jpeg|gif|svg|woff|woff2)$ {
        expires 7d;
        add_header Cache-Control "public, max-age=604800, immutable" always;
        try_files \$uri =404;
    }

    location / {
        try_files \$uri \$uri/ /index.html;
    }
}
EOF
}

wait_for_mysql() {
  local attempt
  for attempt in $(seq 1 60); do
    if compose exec -T mysql sh -lc 'mysqladmin ping -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD" --silent' >/dev/null 2>&1; then
      return 0
    fi
    sleep 2
  done
  log_error "MySQL container did not become ready in time."
  compose logs mysql || true
  exit 1
}

start_mysql_container() {
  log_info "Starting MySQL container..."
  compose up -d mysql
  wait_for_mysql
}

configure_mysql_container() {
  local escaped_password
  escaped_password="$(printf '%s' "${MYSQL_APP_PASSWORD}" | sed "s/'/''/g")"

  compose exec -T mysql sh -lc 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' <<EOF
CREATE DATABASE IF NOT EXISTS \`${MYSQL_APP_DB}\` CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
CREATE USER IF NOT EXISTS '${MYSQL_APP_USER}'@'%' IDENTIFIED BY '${escaped_password}';
ALTER USER '${MYSQL_APP_USER}'@'%' IDENTIFIED BY '${escaped_password}';
GRANT ALL PRIVILEGES ON \`${MYSQL_APP_DB}\`.* TO '${MYSQL_APP_USER}'@'%';
FLUSH PRIVILEGES;
EOF
}

stop_compose_app_services() {
  compose stop backend frontend >/dev/null 2>&1 || true
}

kill_mysql_db_connections() {
  local kill_sql
  kill_sql="$(mktemp)"
  log_info "Terminating existing MySQL sessions connected to ${MYSQL_APP_DB}."
  compose exec -T mysql sh -lc 'exec mysql -N -B -uroot -p"$MYSQL_ROOT_PASSWORD"' <<EOF > "${kill_sql}" || true
SELECT CONCAT('KILL ', id, ';')
FROM information_schema.processlist
WHERE db = '${MYSQL_APP_DB}' AND id <> CONNECTION_ID();
EOF
  if [[ -s "${kill_sql}" ]]; then
    compose exec -T mysql sh -lc 'exec mysql -uroot -p"$MYSQL_ROOT_PASSWORD"' < "${kill_sql}" || true
  fi
  rm -f "${kill_sql}"
}

database_has_tables() {
  local count
  count="$(compose exec -T mysql sh -lc "exec mysql -N -B -uroot -p\"\$MYSQL_ROOT_PASSWORD\" -e \"SELECT COUNT(*) FROM information_schema.tables WHERE table_schema='${MYSQL_APP_DB}' AND table_type='BASE TABLE';\"" 2>/dev/null || printf '0')"
  [[ "${count:-0}" =~ ^[1-9][0-9]*$ ]]
}

transform_sql_file() {
  local source_file="$1"
  local output_file="$2"
  sed "s/\`secmgmt_db\`/\`${MYSQL_APP_DB}\`/g" "${source_file}" > "${output_file}"
}

import_init_sql() {
  local sql_dir="${SCRIPT_DIR}/${SQL_DIR_NAME}"
  local sql_files=()
  local sql_file=""
  local tmp_sql

  if [[ ! -d "${sql_dir}" ]]; then
    log_warn "SQL directory not found: ${sql_dir}, skipping initialization import."
    return
  fi

  while IFS= read -r sql_file; do
    sql_files+=("${sql_file}")
  done < <(find "${sql_dir}" -maxdepth 1 -type f -name '*.sql' | sort)

  if [[ "${#sql_files[@]}" -eq 0 ]]; then
    log_warn "No SQL files found under ${sql_dir}, skipping initialization import."
    return
  fi

  if database_has_tables; then
    if [[ "${REINIT_DB}" -eq 1 ]]; then
      log_warn "Database ${MYSQL_APP_DB} already contains tables, forcing rebuild because --reinit-db is enabled."
    else
      log_info "Database ${MYSQL_APP_DB} already contains tables, skipping initialization SQL import."
      return
    fi
  fi

  stop_compose_app_services
  kill_mysql_db_connections

  tmp_sql="$(mktemp)"
  trap 'rm -f "${tmp_sql}"' RETURN

  for sql_file in "${sql_files[@]}"; do
    log_info "Importing SQL: ${sql_file}"
    transform_sql_file "${sql_file}" "${tmp_sql}"
    log_info "Executing SQL import, please wait..."
    compose exec -T mysql sh -lc 'exec mysql --verbose -uroot -p"$MYSQL_ROOT_PASSWORD"' < "${tmp_sql}"
    log_info "SQL import finished: ${sql_file}"
  done

  rm -f "${tmp_sql}"
  trap - RETURN
}

ensure_runtime_indexes() {
  log_info "Ensuring runtime indexes for smart event and alarm dedup queries."
  compose exec -T mysql sh -lc "exec mysql -uroot -p\"\$MYSQL_ROOT_PASSWORD\" \"${MYSQL_APP_DB}\"" <<'EOF'
SET @idx := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'smart_event' AND index_name = 'ix_smart_event_dedup_time_id'
);
SET @sql := IF(@idx = 0, 'CREATE INDEX ix_smart_event_dedup_time_id ON smart_event (dedup_key, event_time, id)', 'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @idx := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'alarm_record' AND index_name = 'ix_alarm_record_dedup_alarm_time_id'
);
SET @sql := IF(@idx = 0, 'CREATE INDEX ix_alarm_record_dedup_alarm_time_id ON alarm_record (dedup_key, alarm_time, id)', 'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;

SET @idx := (
  SELECT COUNT(*) FROM information_schema.statistics
  WHERE table_schema = DATABASE() AND table_name = 'alarm_record' AND index_name = 'ix_alarm_record_dedup_last_event_time_id'
);
SET @sql := IF(@idx = 0, 'CREATE INDEX ix_alarm_record_dedup_last_event_time_id ON alarm_record (dedup_key, last_event_time, id)', 'SELECT 1');
PREPARE stmt FROM @sql;
EXECUTE stmt;
DEALLOCATE PREPARE stmt;
EOF
}

image_exists() {
  local image_name="$1"
  docker image inspect "${image_name}" >/dev/null 2>&1
}

backend_build_hash() {
  hash_paths \
    "${SCRIPT_DIR}/go.mod" \
    "${SCRIPT_DIR}/go.sum" \
    "${SCRIPT_DIR}/cmd" \
    "${SCRIPT_DIR}/internal" \
    "${SCRIPT_DIR}/Dockerfile.backend" \
    "${SCRIPT_DIR}/.dockerignore" \
    "${SCRIPT_DIR}/third_party/HCNetSDK_Linux64/Header" \
    "${SCRIPT_DIR}/third_party/HCNetSDK_Linux64/Library"
}

frontend_build_hash() {
  hash_paths \
    "${SCRIPT_DIR}/frontend/package.json" \
    "${SCRIPT_DIR}/frontend/package-lock.json" \
    "${SCRIPT_DIR}/frontend/src" \
    "${SCRIPT_DIR}/frontend/public" \
    "${SCRIPT_DIR}/frontend/index.html" \
    "${SCRIPT_DIR}/frontend/vite.config.ts" \
    "${SCRIPT_DIR}/frontend/Dockerfile" \
    "${SCRIPT_DIR}/frontend/.dockerignore"
}

nginx_config_hash() {
  hash_paths "${NGINX_CONF_FILE}"
}

should_rebuild_service() {
  local service_name="$1"
  [[ ",${REBUILD_TARGETS}," == *",${service_name},"* ]]
}

build_service_if_needed() {
  local service_name="$1"
  local image_name="$2"
  local stamp_file="$3"
  local current_hash="$4"
  local previous_hash

  previous_hash="$(read_stamp "${stamp_file}")"

  if should_rebuild_service "${service_name}"; then
    log_info "Selected rebuild enabled, rebuilding ${service_name} image without cache."
    compose build --no-cache "${service_name}"
    write_stamp "${stamp_file}" "${current_hash}"
    return
  fi

  if [[ "${current_hash}" == "${previous_hash}" ]] && image_exists "${image_name}"; then
    log_info "${service_name} image is up to date, skipping rebuild."
    return
  fi

  log_info "${service_name} source changed or image missing, rebuilding image."
  compose build "${service_name}"
  write_stamp "${stamp_file}" "${current_hash}"
}

start_application_stack() {
  local backend_stamp="${CACHE_DIR}/backend-image.sha256"
  local frontend_stamp="${CACHE_DIR}/frontend-image.sha256"
  local nginx_stamp="${CACHE_DIR}/frontend-nginx-conf.sha256"
  local backend_hash
  local frontend_hash
  local nginx_hash
  local previous_nginx_hash
  local nginx_config_changed=0
  local recreate_services=()

  backend_hash="$(backend_build_hash)"
  frontend_hash="$(frontend_build_hash)"
  nginx_hash="$(nginx_config_hash)"
  previous_nginx_hash="$(read_stamp "${nginx_stamp}")"

  build_service_if_needed "backend" "${COMPOSE_PROJECT_NAME}-backend" "${backend_stamp}" "${backend_hash}"
  build_service_if_needed "frontend" "${COMPOSE_PROJECT_NAME}-frontend" "${frontend_stamp}" "${frontend_hash}"

  if should_rebuild_service "backend"; then
    recreate_services+=("backend")
  fi
  if should_rebuild_service "frontend"; then
    recreate_services+=("frontend")
  fi
  if [[ "${nginx_hash}" != "${previous_nginx_hash}" ]]; then
    nginx_config_changed=1
    log_info "Frontend nginx config changed, recreating frontend container."
    if ! should_rebuild_service "frontend"; then
      recreate_services+=("frontend")
    fi
  fi

  if [[ "${#recreate_services[@]}" -gt 0 ]]; then
    log_info "Recreating rebuilt compose services: ${recreate_services[*]}"
    compose up -d --force-recreate "${recreate_services[@]}"
  fi

  compose up -d backend frontend
  if [[ "${nginx_config_changed}" -eq 1 ]]; then
    write_stamp "${nginx_stamp}" "${nginx_hash}"
  fi
}

check_service_active() {
  local service_name="$1"
  if compose ps --status running "${service_name}" | grep -q "${service_name}"; then
    log_info "Compose service ${service_name} is running."
    return 0
  fi
  log_error "Compose service ${service_name} is not running."
  compose ps || true
  compose logs "${service_name}" || true
  exit 1
}

wait_for_http_ready() {
  local label="$1"
  shift
  local attempt
  local max_attempts=30
  local retry_interval_seconds=2

  for attempt in $(seq 1 "${max_attempts}"); do
    if curl -fsS --connect-timeout 3 --max-time 5 "$@" >/dev/null; then
      log_info "${label} is reachable."
      return 0
    fi
    sleep "${retry_interval_seconds}"
  done

  log_error "${label} check failed after ${max_attempts} attempts."
  return 1
}

run_health_checks() {
  local backend_health_url="http://127.0.0.1:${BACKEND_PORT}/healthz"
  local frontend_url="http://127.0.0.1:${FRONTEND_PORT}/"

  log_info "Running deployment health checks."

  if ! compose exec -T mysql sh -lc 'mysqladmin ping -h 127.0.0.1 -uroot -p"$MYSQL_ROOT_PASSWORD" --silent' >/dev/null 2>&1; then
    log_error "MySQL health check failed."
    compose logs mysql || true
    exit 1
  fi
  log_info "MySQL is ready."

  check_service_active backend
  check_service_active frontend

  if ! wait_for_http_ready "Backend health endpoint ${backend_health_url}" "${backend_health_url}"; then
    log_error "Backend health endpoint check failed: ${backend_health_url}"
    compose logs backend || true
    exit 1
  fi

  if [[ "${SERVER_NAME}" == "_" ]]; then
    if ! wait_for_http_ready "Frontend nginx ${frontend_url}" "${frontend_url}"; then
      log_error "Frontend nginx check failed."
      compose logs frontend || true
      exit 1
    fi
  else
    if ! wait_for_http_ready "Frontend nginx ${frontend_url} with Host ${SERVER_NAME}" -H "Host: ${SERVER_NAME}" "${frontend_url}"; then
      log_error "Frontend nginx check failed for Host: ${SERVER_NAME}"
      compose logs frontend || true
      exit 1
    fi
  fi
}

print_summary() {
  local public_url
  public_url="$(guess_public_base_url)"

  echo
  log_info "Docker compose deployment completed."
  echo "Project dir: ${SCRIPT_DIR}"
  echo "Runtime dir: ${RUNTIME_DIR}"
  echo "Compose file: ${COMPOSE_FILE}"
  echo "Compose env: ${COMPOSE_ENV_FILE}"
  echo "Backend env: ${BACKEND_ENV_FILE}"
  echo "Nginx conf: ${NGINX_CONF_FILE}"
  echo "Frontend URL: ${public_url}"
  echo "Backend health: http://127.0.0.1:${BACKEND_PORT}/healthz"
  echo "MySQL database: ${MYSQL_APP_DB}"
  echo "MySQL user: ${MYSQL_APP_USER}"
  echo "MySQL app password: ${MYSQL_APP_PASSWORD}"
  echo "MySQL root password: ${MYSQL_ROOT_PASSWORD}"
  echo "Force install: ${FORCE_INSTALL}"
  echo "Force rebuild: ${FORCE_REBUILD}"
  echo "Rebuild targets: ${REBUILD_TARGETS:-none}"
  echo "APT mirror: ${APT_MIRROR_URL}"
  echo "APT security mirror: ${APT_SECURITY_MIRROR_URL}"
  echo "Debian mirror: ${DEBIAN_APT_MIRROR_URL}"
  echo "Debian security mirror: ${DEBIAN_APT_SECURITY_MIRROR_URL}"
  echo "Go proxy: ${GOPROXY}"
  echo "npm registry: ${NPM_REGISTRY}"
  echo "Docker library mirror: ${DOCKER_LIBRARY_MIRROR}"
  echo
  log_warn "This deployment uses docker compose, and Redis is intentionally excluded for now."
}

main() {
  parse_args "$@"
  require_root
  check_os
  ensure_cache_dir
  install_packages
  ensure_runtime_dirs
  ensure_mysql_passwords
  ensure_docker_running
  log_deployment_mode
  stop_host_conflicting_services
  check_frontend_port_available
  write_backend_env
  write_compose_env
  write_nginx_config
  start_mysql_container
  configure_mysql_container
  import_init_sql
  ensure_runtime_indexes
  start_application_stack
  run_health_checks
  print_summary
}

main "$@"
