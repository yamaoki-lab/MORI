package wifi

import (
	"encoding/json"
	"strconv"
	"strings"
)

type airPortTop struct {
	SPAirPortDataType []struct {
		Interfaces []struct {
			Name                      string            `json:"_name"`
			CurrentNetworkInformation json.RawMessage   `json:"spairport_current_network_information"`
			OtherNetworks             []networkJSON     `json:"spairport_airport_other_local_wireless_networks"`
		} `json:"spairport_airport_interfaces"`
	} `json:"SPAirPortDataType"`
}

// networkJSON は, 現在接続中/周辺ネットワークの両方に共通するフィールド形状
// (SSIDに相当する_nameフィールドはどちらも伏字化されているため, ここには含めない)
type networkJSON struct {
	PHYMode     string `json:"spairport_network_phymode"`
	Security    string `json:"spairport_security_mode"`
	Channel     string `json:"spairport_network_channel"`
	SignalNoise string `json:"spairport_signal_noise"`
}

type currentNetworkExtras struct {
	PHYMode   string
	Security  string
	Channel   string
	SignalDBm *int
}

// parseCurrentNetworkExtras は, `system_profiler SPAirPortDataType -json`の出力から,
// 指定interfaceの現在接続中ネットワークの追加情報 (SSID以外) を取り出す
// SSID (_name) はmacOSでは伏字化されている (実機確認済み) ため意図的に無視する.
// 実際のSSIDはshortcutoutput.go (ショートカット経由) で別途取得する
// 未接続, インタフェースが見つからない, JSON解析失敗時はいずれも (nil, false) を返す
// (エラーとしては扱わない — 呼び出し側にとってはあくまで追加情報のため)
func parseCurrentNetworkExtras(data []byte, iface string) (*currentNetworkExtras, bool) {
	var top airPortTop
	if err := json.Unmarshal(data, &top); err != nil || len(top.SPAirPortDataType) == 0 {
		return nil, false
	}
	for _, ifc := range top.SPAirPortDataType[0].Interfaces {
		if ifc.Name != iface || len(ifc.CurrentNetworkInformation) == 0 {
			continue
		}
		var cur networkJSON
		if err := json.Unmarshal(ifc.CurrentNetworkInformation, &cur); err != nil {
			return nil, false
		}
		return &currentNetworkExtras{
			PHYMode:   cur.PHYMode,
			Security:  cur.Security,
			Channel:   cur.Channel,
			SignalDBm: parseSignalDBm(cur.SignalNoise),
		}, true
	}
	return nil, false
}

// parseOtherNetworks は, `system_profiler SPAirPortDataType -json`の出力から,
// 指定interfaceの周辺ネットワーク (現在接続中を除く) の技術情報を取り出す
// SSIDは現在接続中のもの以外全て伏字化されている (実機確認済み) ため,
// 返すNetworkのSSIDは常に空文字列にする — 名前が分からなくても, 電波強度や
// チャンネル等の他の情報は実データなので, 分からないなりに一覧には載せる
// ("名前不明のネットワークが近くにある" こと自体が情報になるため)
// インタフェースが見つからない場合はnilを返す (エラーにはしない)
func parseOtherNetworks(data []byte, iface string) []Network {
	var top airPortTop
	if err := json.Unmarshal(data, &top); err != nil || len(top.SPAirPortDataType) == 0 {
		return nil
	}
	for _, ifc := range top.SPAirPortDataType[0].Interfaces {
		if ifc.Name != iface {
			continue
		}
		networks := make([]Network, 0, len(ifc.OtherNetworks))
		for _, o := range ifc.OtherNetworks {
			networks = append(networks, Network{
				PHYMode:   o.PHYMode,
				Security:  o.Security,
				Channel:   o.Channel,
				SignalDBm: parseSignalDBm(o.SignalNoise),
			})
		}
		return networks
	}
	return nil
}

// parseSignalDBm は "-51 dBm / -90 dBm" からdBm値 (信号強度側) のみを取り出す
// 失敗時はnil (呼び出し側で "取れなかった" として扱う, エラーにはしない)
func parseSignalDBm(signalNoise string) *int {
	first, _, _ := strings.Cut(signalNoise, "/")
	first = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(first), "dBm"))
	v, err := strconv.Atoi(first)
	if err != nil {
		return nil
	}
	return &v
}
