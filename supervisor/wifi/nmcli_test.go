package wifi

import "testing"

func TestSplitNmcliTerseLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want []string
	}{
		{"通常", "yes:MyNetwork:WPA2:85:40", []string{"yes", "MyNetwork", "WPA2", "85", "40"}},
		{"コロンを含むSSID", `no:Foo\:Bar:WPA2:60:6`, []string{"no", "Foo:Bar", "WPA2", "60", "6"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitNmcliTerseLine(tt.line)
			if len(got) != len(tt.want) {
				t.Fatalf("フィールド数: 期待値 %v, 実際 %v", tt.want, got)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("フィールド[%d]: 期待値 %q, 実際 %q", i, tt.want[i], got[i])
				}
			}
		})
	}
}

func TestParseNmcliWifiList(t *testing.T) {
	const sample = "yes:MyNetwork:WPA2:85:40\nno:OtherNetwork:WPA1 WPA2:50:6\n"
	rows := parseNmcliWifiList([]byte(sample))
	if len(rows) != 2 {
		t.Fatalf("行数: 期待値 2, 実際 %d", len(rows))
	}
	if !rows[0].Active || rows[0].Network.SSID != "MyNetwork" {
		t.Errorf("1行目が想定と違う: %+v", rows[0])
	}
	if rows[1].Active {
		t.Errorf("2行目はActiveでないはず: %+v", rows[1])
	}
	if rows[0].Network.SignalPercent == nil || *rows[0].Network.SignalPercent != 85 {
		t.Errorf("SignalPercent: 期待値 85, 実際 %v", rows[0].Network.SignalPercent)
	}
}
