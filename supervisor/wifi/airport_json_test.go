package wifi

import "testing"

// このJSONは実際にこのマシンで`system_profiler SPAirPortDataType -json`を実行して
// キャプチャした構造そのもの (SSIDは元々macOS自身により伏字化されているため, 伏字化の
// 加工は不要だった)
const sampleAirportJSON = `{
  "SPAirPortDataType" : [
    {
      "spairport_airport_interfaces" : [
        {
          "_name" : "en0",
          "spairport_current_network_information" : {
            "_name" : "<redacted>",
            "spairport_network_channel" : "40 (5GHz, 160MHz)",
            "spairport_network_country_code" : "JP",
            "spairport_network_mcs" : 6,
            "spairport_network_phymode" : "802.11ax",
            "spairport_network_rate" : 1297,
            "spairport_network_type" : "spairport_network_type_station",
            "spairport_security_mode" : "pairport_security_mode_wpa3_transition",
            "spairport_signal_noise" : "-51 dBm / -90 dBm"
          },
          "spairport_airport_other_local_wireless_networks" : [
            {
              "_name" : "<redacted>",
              "spairport_network_channel" : "6 (2GHz, 20MHz)",
              "spairport_network_phymode" : "802.11b/g/n",
              "spairport_network_type" : "spairport_network_type_station",
              "spairport_security_mode" : "spairport_security_mode_wpa2_personal"
            },
            {
              "_name" : "<redacted>",
              "spairport_network_channel" : "40 (5GHz, 160MHz)",
              "spairport_network_phymode" : "802.11a/n/ac/ax",
              "spairport_network_type" : "spairport_network_type_station",
              "spairport_security_mode" : "pairport_security_mode_wpa3_transition",
              "spairport_signal_noise" : "-48 dBm / -90 dBm"
            }
          ]
        }
      ]
    }
  ]
}`

func TestParseCurrentNetworkExtras(t *testing.T) {
	extras, ok := parseCurrentNetworkExtras([]byte(sampleAirportJSON), "en0")
	if !ok {
		t.Fatal("解析に成功するはず")
	}
	if extras.PHYMode != "802.11ax" {
		t.Errorf("PHYMode: 期待値 802.11ax, 実際 %s", extras.PHYMode)
	}
	if extras.Channel != "40 (5GHz, 160MHz)" {
		t.Errorf("Channel: 期待値 \"40 (5GHz, 160MHz)\", 実際 %s", extras.Channel)
	}
	if extras.Security != "pairport_security_mode_wpa3_transition" {
		t.Errorf("Security: 期待値 pairport_security_mode_wpa3_transition, 実際 %s", extras.Security)
	}
	if extras.SignalDBm == nil || *extras.SignalDBm != -51 {
		t.Errorf("SignalDBm: 期待値 -51, 実際 %v", extras.SignalDBm)
	}
}

func TestParseCurrentNetworkExtras_InterfaceNotFound(t *testing.T) {
	if _, ok := parseCurrentNetworkExtras([]byte(sampleAirportJSON), "en9"); ok {
		t.Fatal("存在しないインターフェースなのでfalseになるはず")
	}
}

func TestParseCurrentNetworkExtras_NoCurrentNetwork(t *testing.T) {
	const noCurrent = `{
  "SPAirPortDataType" : [
    { "spairport_airport_interfaces" : [ { "_name" : "en0" } ] }
  ]
}`
	if _, ok := parseCurrentNetworkExtras([]byte(noCurrent), "en0"); ok {
		t.Fatal("current_network_informationが無い場合はfalseになるはず (未接続)")
	}
}

func TestParseOtherNetworks(t *testing.T) {
	networks := parseOtherNetworks([]byte(sampleAirportJSON), "en0")
	if len(networks) != 2 {
		t.Fatalf("件数: 期待値 2, 実際 %d", len(networks))
	}
	for _, n := range networks {
		if n.SSID != "" {
			t.Errorf("SSIDは常に空文字列のはず (名前不明), 実際 %q", n.SSID)
		}
	}
	if networks[0].PHYMode != "802.11b/g/n" || networks[0].SignalDBm != nil {
		t.Errorf("1件目が想定と違う: %+v", networks[0])
	}
	if networks[1].PHYMode != "802.11a/n/ac/ax" || networks[1].SignalDBm == nil || *networks[1].SignalDBm != -48 {
		t.Errorf("2件目が想定と違う: %+v", networks[1])
	}
}

func TestParseOtherNetworks_InterfaceNotFound(t *testing.T) {
	if networks := parseOtherNetworks([]byte(sampleAirportJSON), "en9"); networks != nil {
		t.Fatalf("存在しないインターフェースなのでnilになるはず, 実際 %+v", networks)
	}
}
