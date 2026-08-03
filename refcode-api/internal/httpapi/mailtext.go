package httpapi

import (
	"fmt"
	"strings"
	"time"
)

// 信件文案。錯誤訊息是前端拿 code 自己查語系檔（見 response.go），
// 但信是後端直接寄出去的，前端沒有機會翻譯，所以三種語言都要放在這裡。
//
// 支援的語言跟兩個前端一致：繁中、日文、英文。認不得的一律給繁中。
const (
	localeZhTW = "zh-TW"
	localeJa   = "ja"
	localeEn   = "en"
)

// normalizeLocale 只認語言前綴 —— 前端送來的可能是 zh-TW、zh-Hant-HK、ja-JP、en-GB。
// 中文一律給繁體，沒有簡體語系檔，硬給日文或英文更糟（跟 app 的 i18n/index.ts 同一套判斷）。
func normalizeLocale(raw string) string {
	switch lower := strings.ToLower(strings.TrimSpace(raw)); {
	case strings.HasPrefix(lower, "ja"):
		return localeJa
	case strings.HasPrefix(lower, "en"):
		return localeEn
	default:
		return localeZhTW
	}
}

func resetCodeMail(locale, displayName, code string, ttl time.Duration) (subject, body string) {
	minutes := int(ttl.Minutes())

	switch normalizeLocale(locale) {
	case localeJa:
		return "パスワード再設定の確認コード", fmt.Sprintf(`%s 様

パスワード再設定の確認コードは次のとおりです。

    %s

%d 分以内にアプリに戻って入力してください。このコードは一度しか使用できません。

心当たりのない場合はこのメールを破棄してください。パスワードは変更されません。

—— 推薦碼交流站
`, displayName, code, minutes)

	case localeEn:
		return "Your password reset code", fmt.Sprintf(`Hi %s,

Your password reset code is

    %s

Enter it in the app within %d minutes. The code can only be used once.

If you didn't request this, you can ignore this email — your password won't change.

—— Refcode
`, displayName, code, minutes)

	default:
		return "重設密碼驗證碼", fmt.Sprintf(`%s 您好：

您的重設密碼驗證碼是

    %s

請在 %d 分鐘內回到 App 輸入。驗證碼只能使用一次。

如果這不是您本人的操作，請忽略這封信，您的密碼不會有任何變動。

—— 推薦碼交流站
`, displayName, code, minutes)
	}
}
