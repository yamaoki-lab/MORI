package wifi

import (
	"strconv"
	"strings"
)

// splitNmcliTerseLine は, nmcli -t (terse) 形式の1行を, フィールド内のエスケープ
// (\: と \\) を考慮して分割する. 素朴なstrings.Split(line, ":")では,
// SSIDに":"を含むネットワークで誤動作する
func splitNmcliTerseLine(line string) []string {
	var fields []string
	var cur strings.Builder
	escaped := false
	for _, r := range line {
		switch {
		case escaped:
			cur.WriteRune(r)
			escaped = false
		case r == '\\':
			escaped = true
		case r == ':':
			fields = append(fields, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	fields = append(fields, cur.String())
	return fields
}

// nmcliRow は, `nmcli -t -f ACTIVE,SSID,SECURITY,SIGNAL,CHAN device wifi list`の1行を表す
// ListNetworksとCurrentNetworkの両方が, 同じ1回の実行結果から組み立てられる
// (CurrentNetworkはActiveがtrueの行を探すだけ)
type nmcliRow struct {
	Active  bool
	Network Network
}

func parseNmcliWifiList(output []byte) []nmcliRow {
	var rows []nmcliRow
	for _, line := range strings.Split(string(output), "\n") {
		if line == "" {
			continue
		}
		fields := splitNmcliTerseLine(line)
		if len(fields) < 5 {
			continue // 想定外の行はスキップ (壊れた1行のためにリスト全体を諦めない)
		}
		rows = append(rows, nmcliRow{
			Active: fields[0] == "yes",
			Network: Network{
				SSID:          fields[1],
				Security:      fields[2],
				SignalPercent: parseSignalPercent(fields[3]),
				Channel:       fields[4],
			},
		})
	}
	return rows
}

// parseSignalPercent は, nmcliのSIGNAL列 (0-100の品質値) をパースする
// 失敗時はnil (エラーにはしない)
func parseSignalPercent(s string) *int {
	v, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil
	}
	return &v
}

// filterEnv は, envから指定した名前の環境変数を取り除いた新しいスライスを返す
func filterEnv(env []string, name string) []string {
	prefix := name + "="
	filtered := make([]string, 0, len(env))
	for _, kv := range env {
		if strings.HasPrefix(kv, prefix) {
			continue
		}
		filtered = append(filtered, kv)
	}
	return filtered
}
