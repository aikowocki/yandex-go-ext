#!/bin/sh
set -eu

HARBOR_URL="${HARBOR_URL:-http://localhost:30002}"
HARBOR_ADMIN_USERNAME="${HARBOR_ADMIN_USERNAME:-admin}"
HARBOR_PROJECT="${HARBOR_PROJECT:-gophprofile}"
HARBOR_ROBOT_NAME="${HARBOR_ROBOT_NAME:-gophprofile-local}"
HARBOR_CREDENTIALS_FILE="${HARBOR_CREDENTIALS_FILE:-${XDG_CONFIG_HOME:-$HOME/.config}/gophprofile/harbor-robot.env}"
API_BASE="${HARBOR_URL%/}/api/v2.0"
ROBOT_USERNAME="robot\$${HARBOR_ROBOT_NAME}"

command -v curl >/dev/null 2>&1 || { echo "curl требуется" >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "jq требуется" >&2; exit 1; }

if [ -z "${HARBOR_ADMIN_PASSWORD:-}" ]; then
  [ -t 0 ] || { echo "HARBOR_ADMIN_PASSWORD обязателен для неинтерактивного использования" >&2; exit 1; }
  printf 'Пароль администратора Harbor: '
  stty -echo
  trap 'stty echo 2>/dev/null || true; printf "\n"' EXIT INT TERM
  IFS= read -r HARBOR_ADMIN_PASSWORD
  stty echo
  trap - EXIT INT TERM
  printf '\n'
fi

harbor_get() {
  curl --fail --silent --show-error \
    --user "$HARBOR_ADMIN_USERNAME:$HARBOR_ADMIN_PASSWORD" \
    --retry 30 --retry-delay 2 --retry-connrefused "$1"
}

harbor_post() {
  curl --fail --silent --show-error \
    --user "$HARBOR_ADMIN_USERNAME:$HARBOR_ADMIN_PASSWORD" \
    --header 'Content-Type: application/json' --data "$2" "$1"
}

save_credentials() {
  secret="$1"
  mkdir -p "$(dirname "$HARBOR_CREDENTIALS_FILE")"
  umask 077
  tmp="$(mktemp "$HARBOR_CREDENTIALS_FILE.XXXXXX")"
  trap 'rm -f "$tmp"' EXIT INT TERM
  {
    printf 'HARBOR_REGISTRY=%s\n' "${HARBOR_URL#*://}"
    printf 'HARBOR_ROBOT_USERNAME=%s\n' "$ROBOT_USERNAME"
    printf 'HARBOR_ROBOT_SECRET=%s\n' "$secret"
  } >"$tmp"
  chmod 600 "$tmp"
  mv "$tmp" "$HARBOR_CREDENTIALS_FILE"
  trap - EXIT INT TERM
}

echo "Ожидание API Harbor на $HARBOR_URL..."
harbor_get "$API_BASE/systeminfo" >/dev/null

project_id="$(harbor_get "$API_BASE/projects?name=$HARBOR_PROJECT&page=1&page_size=100" | \
  jq -r --arg name "$HARBOR_PROJECT" 'map(select(.name == $name))[0].project_id // empty')"

if [ -z "$project_id" ]; then
  harbor_post "$API_BASE/projects" "$(jq -cn --arg name "$HARBOR_PROJECT" \
    '{project_name: $name, public: false}')" >/dev/null
  project_id="$(harbor_get "$API_BASE/projects?name=$HARBOR_PROJECT&page=1&page_size=100" | \
    jq -r --arg name "$HARBOR_PROJECT" 'map(select(.name == $name))[0].project_id // empty')"
fi

[ -n "$project_id" ] || { echo "Не удалось получить ID проекта Harbor" >&2; exit 1; }
echo "Harbor готов: $HARBOR_PROJECT"

robots="$(harbor_get "$API_BASE/robots?page=1&page_size=100")"
robot_exists="$(printf '%s' "$robots" | jq -r --arg name "$HARBOR_ROBOT_NAME" \
  'any(.[]; .name == $name or ((.name // "") | endswith($name)))')"

if [ "$robot_exists" = true ]; then
  [ -f "$HARBOR_CREDENTIALS_FILE" ] || {
    echo "Robot существует, но файл учетных данных отсутствует: $HARBOR_CREDENTIALS_FILE" >&2
    echo "Отозовите robot в Harbor перед его повторным созданием." >&2
    exit 1
  }
  secret="$(awk -F= '$1 == "HARBOR_ROBOT_SECRET" {sub(/^[^=]*=/, ""); print; exit}' "$HARBOR_CREDENTIALS_FILE")"
  [ -n "$secret" ] || { echo "Секрет robot отсутствует в файле учетных данных" >&2; exit 1; }
  save_credentials "$secret"
  echo "Учетная запись robot Harbor уже существует: $ROBOT_USERNAME"
else
  payload="$(jq -cn --arg name "$HARBOR_ROBOT_NAME" --arg project "$HARBOR_PROJECT" \
    '{name: $name, description: "Локальная разработка push и pull", duration: -1,
      level: "system", permissions: [{kind: "project", namespace: $project,
        access: [{resource: "repository", action: "push"},
                 {resource: "repository", action: "pull"}]}]}')"
  if robot_response="$(harbor_post "$API_BASE/robots" "$payload" 2>/dev/null)"; then
    secret="$(printf '%s' "$robot_response" | jq -r '(.token // .secret // empty)')"
    [ -n "$secret" ] || { echo "Harbor API не вернул учетные данные robot" >&2; exit 1; }
    save_credentials "$secret"
    echo "Учетная запись robot Harbor создана: $ROBOT_USERNAME"
  elif [ -f "$HARBOR_CREDENTIALS_FILE" ]; then
    secret="$(awk -F= '$1 == "HARBOR_ROBOT_SECRET" {sub(/^[^=]*=/, ""); print; exit}' "$HARBOR_CREDENTIALS_FILE")"
    [ -n "$secret" ] || { echo "Robot уже существует, но его token отсутствует" >&2; exit 1; }
    save_credentials "$secret"
    echo "Учетная запись robot Harbor уже существует: $ROBOT_USERNAME"
  else
    echo "Учетная запись robot Harbor уже существует, но ее файл учетных данных отсутствует: $HARBOR_CREDENTIALS_FILE" >&2
    echo "Отозовите robot в Harbor перед его повторным созданием." >&2
    exit 1
  fi
fi

echo "Файл учетных данных: $HARBOR_CREDENTIALS_FILE"
