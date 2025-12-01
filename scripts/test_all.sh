#!/usr/bin/env bash
set -Eeuo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

print_help() {
  cat <<'EOF'
test_all.sh — запуск тестов для модулей репозитория.

Использование:
  ./scripts/test_all.sh [go-test flags...]

Примеры:
  ./scripts/test_all.sh
  ./scripts/test_all.sh -short -count=1
  ./scripts/test_all.sh -race -covermode=atomic -v
  ./scripts/test_all.sh -run FinishOrder
  ./scripts/test_all.sh -coverpkg=./... -covermode=atomic
  ./scripts/test_all.sh -shuffle=on

Основные флаги go test:
  -short            Запуск коротких тестов (testing.Short()).
  -v                Подробный (verbose) вывод тестов.
  -run REGEXP       Запуск только тестов по шаблону (regexp).
  -race             Детектор гонок данных.
  -count N          Повтор тестов N раз (use -count=1, чтобы отключить кеш).
  -timeout DUR      Лимит на длительность (например, 2m, 30s).
  -covermode MODE   set | count | atomic (для параллельных — atomic).
  -coverpkg PKGS    Пакеты для учёта покрытия (например, ./...).
  -tags TAGS        Build tags.
  -parallel N       Максимум параллельных тестов (t.Parallel()).
  -cpu LIST         Значения GOMAXPROCS (например, 1,2,4).
  -shuffle ARG      Перемешать тесты: on | off | seed.
  -bench REGEXP     Бенчмарки (часто с -benchmem, -benchtime).
EOF
}

if [[ "${1-}" == "-h" || "${1-}" == "--help" ]]; then
  print_help
  exit 0
fi

declare -a EXTRA_FLAGS=("$@")

GREEN="\033[0;32m"; RED="\033[0;31m"; YELLOW="\033[1;33m"; CYAN="\033[0;36m"; BOLD="\033[1m"; NC="\033[0m"
hr() { printf "%s\n" "────────────────────────────────────────────────────────"; }
title() { echo -e "${BOLD}${1}${NC}"; }
ok() { echo -e "${GREEN}✔${NC} $1"; }
fail() { echo -e "${RED}✖${NC} $1"; }
info() { echo -e "${CYAN}›${NC} $1"; }

format_duration() { local s="$1"; (( s<60 )) && printf "%ds" "$s" || printf "%dm%02ds" "$((s/60))" "$((s%60))"; }

declare -a MOD_NAMES=() MOD_STATUS=() MOD_COVER=() MOD_DUR=()

print_pkg_coverage_table() {
  local out_file="$1"
  local rows
  rows="$(awk '
    /coverage: [0-9.]+% of statements/ {
      pct=""; for (i=1;i<=NF;i++) if ($i=="coverage:") pct=$(i+1);
      gsub(/%/,"",pct);
      pkg=$1; if (pkg=="ok") pkg=$2;
      gsub(/^[[:space:]]+|[[:space:]]+$/, "", pkg);
      print pct" "pkg
    }' "${out_file}" | sort -k1,1nr)"
  if [[ -n "${rows}" ]]; then
    printf "%-40s │ %-9s\n" "Package" "Coverage"
    printf "%-40s-┼-%-9s\n" "----------------------------------------" "---------"
    while read -r line; do
      [[ -z "$line" ]] && continue
      local pct_num pkg
      pct_num="${line%% *}"
      pkg="${line#* }"
      awk -v v="${pct_num}" 'BEGIN{exit (v>0.0)?0:1}' || continue
      printf "%-40s │ %s%%\n" "${pkg}" "${pct_num}"
    done <<< "${rows}"
  fi
}

run_module() {
  local name="$1" dir="$2"
  hr; title "🧪 Running tests for ${name}"; info "Dir: ${dir}"; hr
  pushd "${dir}" >/dev/null

  if [[ ! -f go.mod ]]; then
    fail "No go.mod in ${dir}, skipping"
    MOD_NAMES+=("${name}"); MOD_STATUS+=("SKIP"); MOD_COVER+=("n/a"); MOD_DUR+=("0s")
    popd >/dev/null; return 0
  fi

  go mod download >/dev/null 2>&1 || true

  local start_ts end_ts dur_s dur_fmt cover="coverage.out" out="test_output.log"
  local -a test_cmd
  if (( ${#EXTRA_FLAGS[@]} > 0 )); then
    test_cmd=(go test ./... -coverprofile="${cover}" "${EXTRA_FLAGS[@]}")
  else
    test_cmd=(go test ./... -coverprofile="${cover}")
  fi

  start_ts="$(date +%s)"

  if "${test_cmd[@]}" >"${out}" 2>&1; then
    end_ts="$(date +%s)"; start_ts="${start_ts:-$(date +%s)}"; dur_s="$(( end_ts - ${start_ts} ))"; dur_fmt="$(format_duration "${dur_s}")"
    local coverage="n/a"; [[ -f "${cover}" ]] && coverage="$(go tool cover -func="${cover}" | awk '/^total:/ {print $3}')"

    ok "Module ${name} OK"
    info "Total coverage: ${BOLD}${coverage}${NC} | Duration: ${BOLD}${dur_fmt}${NC}"
    print_pkg_coverage_table "${out}"

    MOD_NAMES+=("${name}"); MOD_STATUS+=("OK"); MOD_COVER+=("${coverage}"); MOD_DUR+=("${dur_fmt}")
    popd >/dev/null; return 0
  else
    end_ts="$(date +%s)"; start_ts="${start_ts:-$(date +%s)}"; dur_s="$(( end_ts - ${start_ts} ))"; dur_fmt="$(format_duration "${dur_s}")"
    fail "Module ${name} FAILED (Duration: ${dur_fmt})"
    echo; title "🔎 Test output (${name})"; hr; cat "${out}" || true; echo
    MOD_NAMES+=("${name}"); MOD_STATUS+=("FAIL"); MOD_COVER+=("n/a"); MOD_DUR+=("${dur_fmt}")
    popd >/dev/null; return 1
  fi
}

overall=0
run_module "client" "${ROOT_DIR}/client" || overall=1
run_module "external" "${ROOT_DIR}/external" || overall=1

hr; title "📊 Summary"; hr
printf "%-12s │ %-6s │ %-10s │ %-8s\n" "Module" "Status" "Coverage" "Duration"
printf "%-12s-┼-%-6s-┼-%-10s-┼-%-8s\n" "------------" "------" "----------" "--------"
for i in "${!MOD_NAMES[@]}"; do
  name="${MOD_NAMES[$i]}"; status="${MOD_STATUS[$i]}"; cover="${MOD_COVER[$i]}"; dur="${MOD_DUR[$i]}"
  case "${status}" in
    OK) status="${GREEN}${status}${NC}" ;;
    FAIL) status="${RED}${status}${NC}" ;;
    *) status="${YELLOW}${status}${NC}" ;;
  esac
  printf "%-12s │ %-6b │ %-10s │ %-8s\n" "${name}" "${status}" "${cover}" "${dur}"
done
hr
(( overall == 0 )) && ok "All tests passed" || fail "Some tests failed"
exit "${overall}"
