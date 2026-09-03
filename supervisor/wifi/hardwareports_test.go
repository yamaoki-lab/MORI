package wifi

import "testing"

const sampleHardwarePorts = `Hardware Port: Ethernet Adapter (en3)
Device: en3
Ethernet Address: d2:fe:19:2d:03:89

Hardware Port: USB 10/100/1000 LAN
Device: en5
Ethernet Address: f4:4d:ad:03:5e:6d

Hardware Port: Wi-Fi
Device: en0
Ethernet Address: c4:84:fc:04:90:7a

Hardware Port: Thunderbolt 1
Device: en1
Ethernet Address: 36:5f:ae:a5:f8:00
`

func TestParseHardwarePorts(t *testing.T) {
	iface, err := parseHardwarePorts([]byte(sampleHardwarePorts))
	if err != nil {
		t.Fatalf("予期しないエラー: %v", err)
	}
	if iface != "en0" {
		t.Fatalf("期待値 en0, 実際 %s", iface)
	}
}

func TestParseHardwarePorts_NotFound(t *testing.T) {
	const noWiFi = `Hardware Port: Ethernet Adapter (en3)
Device: en3
Ethernet Address: d2:fe:19:2d:03:89
`
	if _, err := parseHardwarePorts([]byte(noWiFi)); err == nil {
		t.Fatal("Wi-Fiポートが無い場合はエラーになるはず")
	}
}
