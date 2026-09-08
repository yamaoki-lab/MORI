//go:build darwin

package wifi

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// darwinManager は, macOS向けのManager実装
// 全操作をApple純正のCLI (networksetup, system_profiler, scutil, shortcuts) への
// os/exec呼び出しのみで完結させる. cgo経由のCoreWLAN呼び出しやサードパーティの
// ラッパーは意図的に避けている — 攻撃面を絞る方針 (Issue #1) 上, 既にAppleが
// 署名・配布している一次CLIの方が監査しやすいと判断した
//
// SSIDの読み取りはmacOS 15により一律で伏字化されており (networksetup/scutil生ストア/
// system_profiler/wdutil いずれも実機確認済み), 唯一ショートカット.app経由でのみ
// 取得できる (shortcutoutput.go参照). SSID以外の情報 (規格・チャンネル・電波強度・
// セキュリティ方式) は伏字化の対象外で, system_profilerからそのまま取れる
// (airport_json.go参照)
//
// 廃止済みの`airport -s`は実機 (macOS 15.7.7) で非推奨警告のみ出しスキャン結果ゼロ
// だったため使用しない
type darwinManager struct {
	iface string // 例: "en0". 環境依存のため起動時に検出する (ハードコード禁止)
}

func New(ctx context.Context) (Manager, error) {
	iface, err := detectWiFiInterface(ctx)
	if err != nil {
		return nil, fmt.Errorf("Wi-Fiインタフェースの検出に失敗しました: %w", err)
	}
	return &darwinManager{iface: iface}, nil
}

func detectWiFiInterface(ctx context.Context) (string, error) {
	out, err := exec.CommandContext(ctx, "networksetup", "-listallhardwareports").Output()
	if err != nil {
		return "", fmt.Errorf("networksetup -listallhardwareports の実行に失敗しました: %w", err)
	}
	return parseHardwarePorts(out)
}

// ListNetworks: スキャン自体はsystem_profilerで可能 (周辺ネットワークのチャンネル・
// 規格・セキュリティ・電波強度は実データとして返る, 実機確認済み) だが, 各ネットワークの
// SSID (_name) は現在接続中のもの以外全て伏字化されている (ショートカット.app経由でも
// 周辺一覧のSSIDを得る手段は無く, 現在接続中のネットワーク名のみ取得可能)
// 名前が分からなくても電波強度・チャンネル等の他の情報は実データなので, SSIDだけ
// 空文字列のまま一覧に含める ("不明な機体が近くにある" こと自体が情報になるため.
// Network.SSIDが空文字列の項目は名前不明という意味になる)
func (m *darwinManager) ListNetworks(ctx context.Context) ([]Network, error) {
	ssid, err := m.currentSSID(ctx)
	if err != nil {
		return nil, err
	}
	// system_profilerは実測3秒台かかる (shortcuts runも実測5秒台) ため, CurrentNetworkと
	// ListNetworksそれぞれで別々に呼ぶと合計時間が伸びタイムアウトしやすくなる.
	// 1回の実行結果を両方の用途 (現在接続中の追加情報, 周辺一覧) で使い回す
	out, spErr := m.systemProfilerJSON(ctx)

	var networks []Network
	if ssid != "" {
		networks = append(networks, m.buildCurrentNetwork(ssid, out))
	}
	if spErr == nil {
		networks = append(networks, parseOtherNetworks(out, m.iface)...)
	}
	return networks, nil
}

func (m *darwinManager) CurrentNetwork(ctx context.Context) (*Network, error) {
	ssid, err := m.currentSSID(ctx)
	if err != nil {
		return nil, err
	}
	if ssid == "" {
		return nil, nil // 未接続, またはショートカット未セットアップ (どちらもエラーにはしない)
	}
	out, _ := m.systemProfilerJSON(ctx) // 失敗しても追加情報が無いだけ (エラーにはしない)
	network := m.buildCurrentNetwork(ssid, out)
	return &network, nil
}

// currentSSID は, ショートカット経由でSSIDを取得する. 未接続, またはショートカット
// 未セットアップの場合は ("", nil) を返す (エラーではない)
func (m *darwinManager) currentSSID(ctx context.Context) (string, error) {
	os.Remove(currentSSIDFile) // 前回実行結果の誤読みを防ぐ (ショートカット未セットアップ時も含め)
	if err := exec.CommandContext(ctx, "shortcuts", "run", shortcutName).Run(); err != nil {
		return "", fmt.Errorf("shortcuts run %q の実行に失敗しました: %w", shortcutName, err)
	}
	ssid, _ := readCurrentSSID(os.ReadFile)
	return ssid, nil
}

func (m *darwinManager) systemProfilerJSON(ctx context.Context) ([]byte, error) {
	return exec.CommandContext(ctx, "system_profiler", "SPAirPortDataType", "-json").Output()
}

// buildCurrentNetwork は, 取得済みのSSIDと (失敗していてもよい) system_profilerの
// 出力から現在接続中のNetworkを組み立てる. 追加情報が取れなくてもSSIDだけのNetworkを返す
func (m *darwinManager) buildCurrentNetwork(ssid string, systemProfilerOutput []byte) Network {
	network := Network{SSID: ssid}
	if extras, ok := parseCurrentNetworkExtras(systemProfilerOutput, m.iface); ok {
		network.PHYMode = extras.PHYMode
		network.Security = extras.Security
		network.Channel = extras.Channel
		network.SignalDBm = extras.SignalDBm
	}
	return network
}

func (m *darwinManager) Connect(ctx context.Context, ssid, passphrase string) error {
	args := []string{"-setairportnetwork", m.iface, ssid}
	if passphrase != "" {
		args = append(args, passphrase)
	}
	// 引数は常にスライスで渡し, シェル文字列結合は行わない
	// (SSID/パスフレーズをシェル構文として解釈させないため)
	out, err := exec.CommandContext(ctx, "networksetup", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("networksetup -setairportnetwork の実行に失敗しました: %w (output: %s)", err, out)
	}
	// TODO(実装時に実機で要検証): sudoなしで成功するかは未検証 (Web上の情報は割れている)
	return nil
}

func (m *darwinManager) WaitForChange(ctx context.Context) error {
	// Run()自体のエラーは意図的に無視する: 実変化 (exit 0) とscutil自身の60秒
	// タイムアウト (exit 1) のどちらもここでは区別せず, 下記の通りnilを返すため
	_ = exec.CommandContext(ctx, "scutil", "-w",
		"State:/Network/Interface/"+m.iface+"/AirPort", "-t", "60").Run()
	if ctx.Err() != nil {
		return ctx.Err() // 呼び出し元が明示的に停止を要求した
	}
	// 実変化 (exit 0), またはscutil自身の60秒タイムアウト (exit 1, 実測確認済み) の
	// どちらでもnilを返す — 実際に変化したかはwatch.go側がCurrentNetworkで判定する
	return nil
}
