//go:build linux

package wifi

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"
	"time"
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

// nmcliCommand は, nmcliを常にCロケールで実行する *exec.Cmd を返す
// nmcliの出力は一部がロケール依存で, 特に `device wifi list` のACTIVE列は翻訳される
// (ja_JP.UTF-8環境では "yes"/"no" ではなく "はい"/"いいえ" になる. 実機で確認済み).
// 機械的にパースする以上, 実行時ロケールに依存させてはならない
func nmcliCommand(ctx context.Context, args ...string) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "nmcli", args...)
	// 環境変数の配列に同名キーが複数含まれた場合にどちらが有効になるかはlibc実装依存
	// (glibcのgetenvは先勝ちとされる) のため, 単純にLC_ALL=Cを末尾へ追加するだけでは,
	// 呼び出し元の環境で既にLC_ALLが設定されていた場合に上書きできない可能性がある.
	// 既存のLC_ALLを除いてから足すことで, 実装依存の挙動に関わらず確実に1個だけにする
	cmd.Env = append(filterEnv(os.Environ(), "LC_ALL"), "LC_ALL=C")
	return cmd
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
	out, err := nmcliCommand(ctx, "-t", "-f", nmcliFields, "device", "wifi", "list").Output()
	if err != nil {
		// nmcliは失敗理由をstderrにしか書かないため, %wだけでは "exit status 1" しか残らない
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("nmcli device wifi list の実行に失敗しました: %w (stderr: %s)", err, bytes.TrimSpace(exitErr.Stderr))
		}
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
	out, err := nmcliCommand(ctx, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("nmcli device wifi connect の実行に失敗しました: %w (output: %s)", err, bytes.TrimSpace(out))
	}
	// 研究室実機で検証済み: 一般ユーザ権限のままでは polkit の
	// org.freedesktop.NetworkManager.network-control が "auth" 判定となり,
	// "Not authorized to control networking." (exit status 4) で失敗する.
	// 開放AP (TELLO系, パスフレーズなし) でも同様に拒否されるため,
	// 実運用にはpolkitルールの追加かroot実行が必須.
	// (デプロイ/ホスト設定側の課題であり, このコードでは解決できない)
	return nil
}

// nmcliMonitorStatusPrefix は, `nmcli monitor` が起動直後に必ず1行出力する
// デーモンの稼働状態表示 ("NetworkManager is running") の接頭辞
// これは "変化" ではないため, WaitForChangeの復帰条件から除外する必要がある
const nmcliMonitorStatusPrefix = "NetworkManager is "

// nmcliMonitorSettle は, 最初の変化イベントを受け取ってから復帰するまでの待ち時間
// 状態遷移のバースト (実機実測で約250ms) が収まるのを待つためのもの
const nmcliMonitorSettle = 1 * time.Second

func (m *linuxManager) WaitForChange(ctx context.Context) error {
	cmd := nmcliCommand(ctx, "monitor")
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

	defer func() {
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// 1行読めたら "何か変化した" シグナルとして扱う. 内容自体はパースしない
	// (nmcli monitorの出力は構造化パースに向かないとされているため.
	// 実際の値取得は呼び出し側がCurrentNetworkで行う)
	// ただし起動時の状態表示だけは無条件に出るため, 読み飛ばさないと
	// WaitForChangeが常に即時復帰し, 呼び出し元がビジーループに陥る (実機で確認済み)
	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		if strings.HasPrefix(scanner.Text(), nmcliMonitorStatusPrefix) {
			continue
		}
		// 状態遷移は複数行のバーストで届き, 先頭行の時点ではまだ遷移の途中で
		// CurrentNetworkは最終状態を返さない (実機実測: 接続は "connection profile changed"
		// から "connected" まで約250msの間に7行が流れる).
		// 先頭行で即座に復帰すると, 呼び出し元がCurrentNetworkを読む時点では
		// まだ "connecting" のため変化を検出できず, さらにmonitorを起動し直す間に
		// 残りのバーストを取りこぼして接続を見逃す (実機で確認済み).
		// バーストが収まるのを待ってから復帰する
		select {
		case <-time.After(nmcliMonitorSettle):
		case <-ctx.Done():
			return ctx.Err()
		}
		return nil
	}

	// ここに来たのはEOF, つまりnmcli monitorが変化を伝えずに終了した場合
	// nilを返すと呼び出し元が即座に再突入してビジーループになるため, 必ずエラーにする
	if ctx.Err() != nil {
		return ctx.Err() // 呼び出し元が明示的に停止を要求した
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("nmcli monitor の出力読み取りに失敗しました: %w", err)
	}
	return errors.New("nmcli monitor が予期せず終了しました")
}
