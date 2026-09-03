//go:build linux

package wifi

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"syscall"
)

// linuxManager は, Linux (研究室実機, 本番対象) 向けのManager実装
// nmcli (NetworkManager) 前提の一次実装. 実機での検証は未実施 (研究室訪問時に
// 検証予定, macOS開発機ではLinux実機での挙動そのものは確認できない)
type linuxManager struct{}

const nmcliFields = "ACTIVE,SSID,SECURITY,SIGNAL,CHAN"

func New(ctx context.Context) (Manager, error) {
	if _, err := exec.LookPath("nmcli"); err != nil {
		return nil, fmt.Errorf("nmcliが見つかりません (NetworkManagerが未インストールの可能性): %w", err)
	}
	return &linuxManager{}, nil
}

func (m *linuxManager) ListNetworks(ctx context.Context) ([]Network, error) {
	rows, err := m.wifiList(ctx)
	if err != nil {
		return nil, err
	}
	networks := make([]Network, 0, len(rows))
	for _, row := range rows {
		networks = append(networks, row.Network)
	}
	return networks, nil
}

func (m *linuxManager) CurrentNetwork(ctx context.Context) (*Network, error) {
	rows, err := m.wifiList(ctx)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row.Active {
			network := row.Network
			return &network, nil
		}
	}
	return nil, nil // 未接続 (エラーではない)
}

func (m *linuxManager) wifiList(ctx context.Context) ([]nmcliRow, error) {
	out, err := exec.CommandContext(ctx, "nmcli", "-t", "-f", nmcliFields, "device", "wifi", "list").Output()
	if err != nil {
		return nil, fmt.Errorf("nmcli device wifi list の実行に失敗しました: %w", err)
	}
	return parseNmcliWifiList(out), nil
}

func (m *linuxManager) Connect(ctx context.Context, ssid, passphrase string) error {
	args := []string{"device", "wifi", "connect", ssid}
	if passphrase != "" {
		args = append(args, "password", passphrase)
	}
	// 引数は常にスライスで渡し, シェル文字列結合は行わない
	out, err := exec.CommandContext(ctx, "nmcli", args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nmcli device wifi connect の実行に失敗しました: %w (output: %s)", err, out)
	}
	// TODO(要検証, 研究室訪問時): root権限またはpolkit設定なしでも成功するかは未検証
	// (デプロイ/ホスト設定側の課題であり, このコードでは解決できない前提)
	return nil
}

func (m *linuxManager) WaitForChange(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "nmcli", "monitor")
	// 親プロセス(このGoプロセス)がSIGKILL等で不正終了しても, カーネルが子(nmcli monitor)を
	// 道連れに終了させる (孤児プロセス対策). Linux固有の機構
	cmd.SysProcAttr = &syscall.SysProcAttr{Pdeathsig: syscall.SIGKILL}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("nmcli monitor の起動準備に失敗しました: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("nmcli monitor の起動に失敗しました: %w", err)
	}

	// 1行読めたら "何か変化した" シグナルとして扱う. 内容自体はパースしない
	// (nmcli monitorの出力は構造化パースに向かないとされているため.
	// 実際の値取得は呼び出し側がCurrentNetworkで行う)
	bufio.NewReader(stdout).ReadString('\n')

	cmd.Process.Kill()
	cmd.Wait()

	if ctx.Err() != nil {
		return ctx.Err() // 呼び出し元が明示的に停止を要求した
	}
	return nil
}
