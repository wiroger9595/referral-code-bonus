#!/usr/bin/env bash
# 出版本用的統一入口，跟 dev.sh 分開：dev.sh 動的都是本機的東西，這支產出的是
# 要傳上商店的包，跑錯了會傳出去收不回來。
# 一樣寫成 bash 3.2 相容（macOS 內建的還是 3.2）。
#
# 商店對版本號只有一條硬規則：每次上傳的建置編號都要比上一次大，而且不能重複
# （Play 是 versionCode、App Store 是 Build）。忘記加就是上傳到最後一步才被擋，
# 所以這支腳本預設自己加，不靠人記。
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
APP="$ROOT/refcode-app"
GRADLE="$APP/android/app/build.gradle"
PBXPROJ="$APP/ios/App/App.xcodeproj/project.pbxproj"
AAB="$APP/android/app/build/outputs/bundle/release/app-release.aab"

usage() {
  cat <<EOF
用法：./release.sh <平台> [選項]

  android          版本號 +1 → 打包網頁 → 同步原生專案 → 產出 AAB
  ios              版本號 +1 → 打包網頁 → 同步原生專案 → 開 Xcode（Archive 要自己按）
  all              兩個平台都做
  （不給平台）      只顯示目前的版本號，不動任何東西

選項：
  --name <字串>    同時改顯示版本（Android versionName／iOS Marketing Version），
                   例如 --name 1.1。有新功能才需要，改 bug 不必動。
  --no-bump        不要自動加建置編號。上一次 build 失敗、要用同一個號碼重跑時用。

例：
  ./release.sh android              下一個版本代碼，產出 AAB
  ./release.sh all --name 1.1       兩個平台都出，顯示版本一起推到 1.1
EOF
}

# 兩個平台的建置編號故意共用同一個數字。它們各自獨立遞增也可以，但那要記兩套，
# 而「這包是哪一版」在對照 crash log 時是最常問的問題，統一之後看一眼就對得起來。
current_build() { sed -n 's/.*versionCode \([0-9]*\).*/\1/p' "$GRADLE" | head -1; }
current_name()  { sed -n 's/.*versionName "\(.*\)".*/\1/p' "$GRADLE" | head -1; }
current_ios_build() { sed -n 's/.*CURRENT_PROJECT_VERSION = \([0-9]*\);.*/\1/p' "$PBXPROJ" | head -1; }

show_versions() {
  echo "  Android  versionCode $(current_build)   versionName $(current_name)"
  echo "  iOS      Build $(current_ios_build)   Version $(sed -n 's/.*MARKETING_VERSION = \(.*\);.*/\1/p' "$PBXPROJ" | head -1)"
}

# sed -i 在 macOS 要給備份字尾，給空字串代表不留備份（GNU sed 的寫法在這裡會壞）。
set_build() {
  sed -i '' "s/versionCode [0-9]*/versionCode $1/" "$GRADLE"
  # pbxproj 裡 Debug 與 Release 各有一份，兩份都要換 —— 只換一份的話
  # Archive 出來的還是舊號碼，而那正是會被上傳的那一個。
  sed -i '' "s/CURRENT_PROJECT_VERSION = [0-9]*;/CURRENT_PROJECT_VERSION = $1;/g" "$PBXPROJ"
}

set_name() {
  sed -i '' "s/versionName \".*\"/versionName \"$1\"/" "$GRADLE"
  sed -i '' "s/MARKETING_VERSION = .*;/MARKETING_VERSION = $1;/g" "$PBXPROJ"
}

platform=""
new_name=""
bump=1

while [ $# -gt 0 ]; do
  case "$1" in
    android|ios|all) platform="$1" ;;
    --name) shift; new_name="${1:-}"; [ -n "$new_name" ] || { echo "--name 後面要給版本字串，例如 --name 1.1"; exit 1; } ;;
    --no-bump) bump=0 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "不認得的參數：$1"; echo; usage; exit 1 ;;
  esac
  shift
done

if [ -z "$platform" ]; then
  echo "目前的版本號："
  show_versions
  echo
  usage
  exit 0
fi

if [ "$bump" -eq 1 ]; then
  next=$(( $(current_build) + 1 ))
  set_build "$next"
  echo "▸ 建置編號 → $next"
fi
if [ -n "$new_name" ]; then
  set_name "$new_name"
  echo "▸ 顯示版本 → $new_name"
fi

# 網頁的部分兩個平台共用，只打包一次。npm run build 會把 API 位址寫死成正式環境
# （見 refcode-app/package.json），這對要送商店的包才是對的。
echo "▸ 打包網頁"
cd "$APP"
npm run build

echo "▸ 同步原生專案"
if [ "$platform" = "all" ]; then
  npx cap sync
else
  npx cap sync "$platform"
fi

if [ "$platform" = "android" ] || [ "$platform" = "all" ]; then
  echo "▸ 產出 AAB"
  cd "$APP/android"
  ./gradlew bundleRelease
  echo
  echo "AAB：$AAB"
  echo "上傳前確認 Play Console 那個版本裡只有這一包，舊的移掉。"
fi

if [ "$platform" = "ios" ] || [ "$platform" = "all" ]; then
  echo
  # Archive 到上傳這一段沒有自動化：憑證、描述檔與 App Store Connect 的驗證
  # 都要人在場點過，寫成腳本反而會在中途卡著等一個看不見的對話框。
  echo "iOS 接下來要自己按：Xcode → Product → Archive → Distribute App"
  open "$APP/ios/App/App.xcworkspace"
fi

echo
echo "完成。目前版本號："
show_versions
