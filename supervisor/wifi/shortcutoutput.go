package wifi

import "strings"

// currentSSIDFile は, 手動セットアップ済みの "ネットワーク名を取得" ショートカットが
// SSIDを書き出す先. macOSはSSIDの読み取りを一律で伏字化する (system_profiler/
// networksetup/scutil/wdutil いずれも実機確認済み) ため, 正規のApple製アプリである
// ショートカット.app経由でしか取得できない. ショートカット内のシェルスクリプト
// アクション自体は実際のSSIDを扱えている (プレビューでも確認できる) が, その実行結果は
// ショートカット側のプレビュー上で完結しており, `shortcuts run`を叩いたターミナルの
// 標準出力へは ("停止して出力" を挟んでも) 届かない — 意図的な遮断というより,
// 実行される場所が違うため単純に繋がっていない, という理解. そのためファイルへの
// 書き出しという, 経路に依存しない受け渡し方法を使っている (実機確認済み)
// ショートカット自体のセットアップ手順は docs/setup-macos.md 参照
const currentSSIDFile = "/tmp/mori_current_ssid.txt"

const shortcutName = "ネットワーク名を取得"

// readCurrentSSID は, ショートカットが書き出したファイルからSSIDを読む
// ファイルが存在しない, または中身が空の場合は未接続とみなし ("", false) を返す
// (エラーではない — ショートカット未セットアップの場合もこの経路を通る)
func readCurrentSSID(readFile func(string) ([]byte, error)) (string, bool) {
	b, err := readFile(currentSSIDFile)
	if err != nil {
		return "", false
	}
	ssid := strings.TrimSpace(string(b))
	if ssid == "" {
		return "", false
	}
	return ssid, true
}
