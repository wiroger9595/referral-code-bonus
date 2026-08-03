#!/usr/bin/env bash
# 四個模組是各自獨立的 repo，這支腳本只是本機開發時的統一入口，不屬於任何一個 repo。
# 寫成 bash 3.2 相容（macOS 內建的還是 3.2，沒有關聯陣列可用）。
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LOGS="$ROOT/.logs"
ORDER="api admin web app"

dir_of() {
  case "$1" in
    api)   echo "refcode-api" ;;
    admin) echo "refcode-admin" ;;
    web)   echo "refcode-web" ;;
    app)   echo "refcode-app" ;;
  esac
}

port_of() {
  case "$1" in
    api)   echo 7802 ;;
    admin) echo 5173 ;;
    web)   echo 3000 ;;
    app)   echo 5174 ;;
  esac
}

usage() {
  cat <<EOF
用法：./dev.sh <指令>

  status              看四個 port 有沒有在聽
  api|admin|web|app   單獨前景啟動，Ctrl-C 停止
  all                 四個背景啟動，log 寫進 .logs/
  stop [模組...]      不給模組就四個全停
  logs <模組>         跟蹤某個模組的 log
EOF
}

port_pid() { lsof -ti:"$1" 2>/dev/null | head -1; }

ensure_deps() {
  mod=$1
  dir="$ROOT/$(dir_of "$mod")"

  if [ ! -f "$dir/.env" ] && [ -f "$dir/.env.example" ]; then
    cp "$dir/.env.example" "$dir/.env"
    echo "已建立 $(dir_of "$mod")/.env"
  fi

  if [ "$mod" = "api" ]; then
    (cd "$dir" && go mod download)
  elif [ ! -d "$dir/node_modules" ]; then
    echo "安裝 $(dir_of "$mod") 相依..."
    (cd "$dir" && npm install)
  fi
}

start_fg() {
  mod=$1
  dir="$ROOT/$(dir_of "$mod")"
  ensure_deps "$mod"
  # 全形括號緊接在 $mod 後面會被 bash 3.2 當成變數名的一部分，一律加大括號。
  echo "啟動 ${mod}（port $(port_of "$mod")）"
  if [ "$mod" = "api" ]; then
    (cd "$dir" && go run ./cmd/api)
  else
    (cd "$dir" && npm run dev)
  fi
}

start_bg() {
  mod=$1
  dir="$ROOT/$(dir_of "$mod")"
  port=$(port_of "$mod")

  if [ -n "$(port_pid "$port")" ]; then
    echo "$mod 已經在 port $port 跑著，跳過"
    return
  fi

  ensure_deps "$mod"
  mkdir -p "$LOGS"

  if [ "$mod" = "api" ]; then
    (cd "$dir" && nohup go run ./cmd/api >"$LOGS/$mod.log" 2>&1 &)
  else
    (cd "$dir" && nohup npm run dev >"$LOGS/$mod.log" 2>&1 &)
  fi
  echo "$mod 啟動中（port ${port}，log: .logs/$mod.log）"
}

cmd_status() {
  for mod in $ORDER; do
    port=$(port_of "$mod")
    pid=$(port_pid "$port")
    if [ -n "$pid" ]; then
      printf '  %-6s port %-5s ✓ 執行中（pid %s）\n' "$mod" "$port" "$pid"
    else
      printf '  %-6s port %-5s ✗ 沒在跑\n' "$mod" "$port"
    fi
  done
}

cmd_stop() {
  mods="$*"
  [ -z "$mods" ] && mods="$ORDER"

  for mod in $mods; do
    port=$(port_of "$mod")
    if [ -z "$port" ]; then
      echo "不認得的模組：$mod"
      continue
    fi
    pid=$(port_pid "$port")
    if [ -z "$pid" ]; then
      echo "$mod 沒在跑"
      continue
    fi

    kill "$pid" 2>/dev/null

    # api 有優雅關閉（最長 5 秒，等連線處理完），這裡要等 port 真的空出來再回傳，
    # 不然緊接著的 start 會撞 address already in use —— kill 送出訊號不代表 port 已經放掉。
    waited=0
    while [ -n "$(port_pid "$port")" ] && [ "$waited" -lt 5 ]; do
      sleep 1
      waited=$((waited + 1))
    done

    still=$(port_pid "$port")
    if [ -n "$still" ]; then
      kill -9 "$still" 2>/dev/null
      echo "已強制停止 ${mod}（pid ${still}，優雅關閉超過 15 秒沒結束）"
    else
      echo "已停止 ${mod}（pid ${pid}）"
    fi
  done
}

case "${1:-}" in
  status) cmd_status ;;
  all)    for mod in $ORDER; do start_bg "$mod"; done; echo; cmd_status ;;
  stop)   shift; cmd_stop "$@" ;;
  logs)   tail -f "$LOGS/${2:-api}.log" ;;
  api|admin|web|app) start_fg "$1" ;;
  *)      usage; exit 1 ;;
esac
