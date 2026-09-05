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
  ios [裝置關鍵字]    在 iOS 模擬器上跑 app，改前端會自動更新（Ctrl-C 結束）
  ios-log             跟蹤模擬器裡 app 的 console log，跟 ios 分開開一個 terminal 跑
  android-reverse     把手機的 localhost 轉回這台電腦（adb reverse），實機連本機 API 用
                       （手機重開、拔插 USB、adb server 重啟之後要重跑）
EOF
}

port_pid() { lsof -ti:"$1" 2>/dev/null | head -1; }

# 佔著 port 的那個 process 是不是這個 workspace 起的。判斷看它的工作目錄 ——
# `go run` 真正 listen 的是編在 /var/folders 底下的暫存執行檔，從路徑完全看不出
# 是誰的，但 cwd 一定還留在 refcode-* 裡面。
port_pid_is_ours() {
  cwd=$(lsof -a -p "$1" -d cwd -Fn 2>/dev/null | sed -n 's/^n//p' | head -1)
  case "$cwd" in
    "$ROOT"|"$ROOT"/*) return 0 ;;
    *) return 1 ;;
  esac
}

# 撞埠時要停掉舊的才啟動得起來。舊的如果是這個 workspace 自己留下的（多半是
# `./dev.sh all` 背景起的，terminal 關掉之後變成孤兒，Ctrl+C 碰不到），就直接停掉
# 再繼續 —— 那是使用者本來就要的結果，多問一次只是多一個步驟。
# 不是我們的就停在這裡：別人的 process 不該被這個腳本殺掉。
free_port_or_die() {
  mod=$1
  port=$(port_of "$mod")
  pid=$(port_pid "$port")
  [ -z "$pid" ] && return 0

  if port_pid_is_ours "$pid"; then
    echo "port $port 還被舊的 ${mod} 佔著（pid ${pid}），先停掉它"
    cmd_stop "$mod" >/dev/null
    return 0
  fi

  echo "port $port 被別的程式佔著，不是這個 workspace 起的，我不會去動它："
  echo "  pid $pid  $(ps -o command= -p "$pid" 2>/dev/null | head -1)"
  echo
  echo "確定要停掉的話：kill $pid"
  exit 1
}

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
  free_port_or_die "$mod"
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

  pid=$(port_pid "$port")
  if [ -n "$pid" ]; then
    # 背景啟動不自己停舊的：`./dev.sh all` 的用意是「把沒跑的補起來」，
    # 已經在跑的重啟一次反而會打斷正在用它的人。
    if port_pid_is_ours "$pid"; then
      echo "$mod 已經在 port $port 跑著，跳過（要重啟：./dev.sh stop $mod && ./dev.sh ${mod}）"
    else
      echo "$mod 沒起來：port $port 被別的程式佔著"
      echo "  pid $pid  $(ps -o command= -p "$pid" 2>/dev/null | head -1)"
    fi
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

# 在 iOS 模擬器上跑 app，並且讓它去載本機的 vite —— 改前端存檔就自動更新，
# 不用重跑一次 cap run。
#
# 幾個一定要指定的東西，預設值都是錯的：
#   --port  Capacitor CLI 預設 3000，那是 refcode-web 的 port，不給的話
#           模擬器裡會載到官網。
#   --host  預設是這台機器的區網 IP，那需要 vite 監聽 0.0.0.0。模擬器跟 Mac
#           共用網路，用 localhost 就通，也省得改 vite 的設定。
#           （實機不適用，那時才需要區網 IP + vite --host。）
cmd_ios() {
  dir="$ROOT/refcode-app"
  port=$(port_of app)

  ensure_deps app

  # vite 沒在跑就先背景起來 —— cap run 只負責把 app 指向這個位址，
  # 它不會幫你啟動 dev server，少了這步模擬器裡會是一片白。
  #
  # 是我們起的就要負責收掉：nohup 起來的 vite 結束時不會跟著走，會變成沒有
  # 父行程的孤兒繼續佔著 5174，下次啟動就撞埠。本來就在跑的不動它 ——
  # 那是使用者自己開的視窗，不該被這個指令關掉。
  started_vite=0
  if [ -z "$(port_pid "$port")" ]; then
    mkdir -p "$LOGS"
    (cd "$dir" && nohup npm run dev >"$LOGS/app.log" 2>&1 &)
    started_vite=1
    trap 'if [ "$started_vite" = 1 ]; then pid=$(port_pid "$port"); [ -n "$pid" ] && kill "$pid" 2>/dev/null && echo "已收掉這次啟動的 vite（pid ${pid}）"; fi' EXIT INT TERM
    echo "vite 啟動中（port ${port}，log: .logs/app.log）"
    waited=0
    while [ -z "$(port_pid "$port")" ] && [ "$waited" -lt 30 ]; do
      sleep 1
      waited=$((waited + 1))
    done
    if [ -z "$(port_pid "$port")" ]; then
      echo "vite 沒起來，看一下 .logs/app.log"
      exit 1
    fi
  else
    echo "vite 已經在 port ${port} 跑著，沿用"
  fi

  # 原生殼裡的 plugin 與設定是 sync 進去的，跟 live reload 無關但會過期。
  # 這步很快，每次跑一下省得之後裝了新 plugin 找不到原因。
  (cd "$dir" && npx cap sync ios) || exit 1

  target=""
  if [ -n "${1:-}" ]; then
    # 用關鍵字挑模擬器，例如 ./dev.sh ios 17e
    target=$(xcrun simctl list devices available 2>/dev/null \
      | grep -i "$1" | head -1 | sed -E 's/.*\(([0-9A-F-]{36})\).*/\1/')
    if [ -z "$target" ]; then
      echo "找不到符合「$1」的模擬器。可用的："
      xcrun simctl list devices available | grep -E "^\s+(iPhone|iPad)" | head -10
      exit 1
    fi
  fi

  echo
  echo "接下來畫面會停在「App running with live reload」——那是正常的，別關掉。"
  echo "改 refcode-app/src 底下的東西存檔就會自動更新。**要用 Ctrl-C 結束**，"
  echo "它才會把原生專案的設定改回來（硬殺的話下次 build 出來的 app 會一直想連這台）。"
  echo

  if [ -n "$target" ]; then
    (cd "$dir" && npx cap run ios -l --host localhost --port "$port" --target "$target")
  else
    (cd "$dir" && npx cap run ios -l --host localhost --port "$port")
  fi
}

# cap run ios -l 那個 terminal 只印 Capacitor CLI 自己的建置/部署訊息，WebView
# 裡的 console.log／console.error 不會出現在那邊。Capacitor 的 iOS bridge 會把
# 那些訊息轉成 NSLog（前綴通常是 ⚡️），所以用系統的 log stream 抓，跟 ios 分開
# 開一個 terminal 跑。認的是「目前開著的模擬器」，開兩台以上會不知道抓哪台。
cmd_ios_log() {
  xcrun simctl spawn booted log stream --level debug --predicate 'process == "App"'
}


# 手機上的 localhost 指的是手機自己，adb reverse 把它從 USB 那條線轉回這台電腦。
# 這樣 .env 與 network_security_config 都固定寫 localhost，換 WiFi、換網段都不用再
# 動任何檔案 —— 以前那兩個地方寫死區網 IP，IP 一變就是整個 app 連不上，而且症狀
# 看不出跟 IP 有關。轉發規則存在 adb daemon 裡、不是永久的：手機重開、拔插 USB、
# adb server 重啟就沒了，重跑這個指令即可。
cmd_android_reverse() {
  adb=$(command -v adb 2>/dev/null || echo "$HOME/Library/Android/sdk/platform-tools/adb")
  if [ ! -x "$adb" ]; then
    echo "找不到 adb。裝過 Android Studio 的話它在 ~/Library/Android/sdk/platform-tools/"
    exit 1
  fi

  count=$("$adb" devices | awk '$2 == "device" { n++ } END { print n + 0 }')
  if [ "$count" = "0" ]; then
    echo "沒有連著的裝置。USB 接上（或先用無線偵錯配對），手機上的偵錯授權要允許這台電腦。"
    exit 1
  fi
  # 多台裝置時 adb reverse 不會自己挑，會直接失敗（要帶 -s）。與其猜一台，
  # 不如停在這裡讓使用者決定 —— 設錯機器的症狀跟沒設一樣難查。
  if [ "$count" != "1" ]; then
    echo "接了 ${count} 台裝置，adb reverse 不知道要設哪一台。拔到只剩一台再跑："
    "$adb" devices
    exit 1
  fi

  # vite 也一起轉：live reload 就能用 --host localhost，不必再讓手機去找區網 IP。
  for port in $(port_of api) $(port_of app); do
    "$adb" reverse "tcp:${port}" "tcp:${port}" >/dev/null || exit 1
  done

  echo "已把手機的 localhost 轉回這台電腦："
  "$adb" reverse --list
  echo
  echo "API（$(port_of api)）與 vite（$(port_of app)）都通了。"
  echo
  echo "要裝到實機測本機 API：npm run build:local（不帶環境變數的 vite build，會讀 .env"
  echo "的 localhost:7802）→ npx cap sync android → 重裝。不要用 npm run build ——"
  echo "那支把正式站的 API URL 寫死在指令裡，出來的包一律連正式環境（那是包版用的）。"
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
  ios)    shift; cmd_ios "${1:-}" ;;
  ios-log) cmd_ios_log ;;
  android-reverse) cmd_android_reverse ;;
  api|admin|web|app) start_fg "$1" ;;
  *)      usage; exit 1 ;;
esac
