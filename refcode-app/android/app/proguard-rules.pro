# Capacitor 的原生層幾乎都是被反射或 JS 橋接叫起來的，R8 看不到呼叫點。
# 好消息是大部分規則不用自己寫：@capacitor/android 的 consumer rules 已經 keep 了
# 所有繼承 com.getcapacitor.Plugin 的類別（capacitor.plugins.json 裡那七個都是），
# @capgo/capacitor-social-login 也自帶一份涵蓋 Google 登入與 CustomTabs 的規則。
# 這個檔只補那兩份沒涵蓋、而這個 app 會踩到的部分。

# JS 呼叫原生的入口。方法名被改掉時 WebView 那側是靜默失敗 —— 找不到方法就當作
# undefined，不會丟例外也不會進 logcat，是最難查的一種壞法。AGP 的預設檔也有
# 這條，明寫一份不吃虧。
-keepclassmembers class * {
    @android.webkit.JavascriptInterface <methods>;
}

# AndroidManifest 用字串指名的類別，R8 從程式碼追不到它。
-keep class com.referra.app.MainActivity { *; }

# 當機堆疊要看得懂行號。少了這兩行，Play Console 的當機報告只剩混淆後的名字，
# 而 Play 新的記憶體門檻正是拿當機率在判定的 —— 看不懂堆疊就修不掉。
# 反混淆靠 build 產生的 mapping.txt，AAB 上傳時會一起帶上去。
-keepattributes SourceFile,LineNumberTable
-renamesourcefileattribute SourceFile
