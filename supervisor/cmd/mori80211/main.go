// Command mori80211 は, wifiパッケージの手動テスト用CLI
// このインタフェースには現時点で利用者 (console等) が存在しないため,
// 人間が動作確認するための最小限のツールとして用意する
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/yamaoki-lab/MORI/supervisor/wifi"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "list":
		runWithTimeout(os.Args[2:], runList)
	case "current":
		runWithTimeout(os.Args[2:], runCurrent)
	case "connect":
		runConnect(os.Args[2:])
	case "watch":
		runWatch(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, `使い方:
  mori80211 list                          近隣ネットワーク一覧を表示
  mori80211 current                       現在接続中のネットワークを表示
  mori80211 connect <SSID> [passphrase]   指定SSIDへの接続を要求
  mori80211 watch                         接続先の変化をリアルタイムに表示し続ける (^Cで終了)
共通フラグ (watch以外): -timeout duration (デフォルト20s)`)
}

func runWithTimeout(args []string, fn func(ctx context.Context, mgr wifi.Manager) error) {
	fs := flag.NewFlagSet("", flag.ExitOnError)
	timeout := fs.Duration("timeout", 20*time.Second, "操作のタイムアウト (system_profiler/shortcuts runは合わせて実測8秒程度かかるため余裕を持たせている)")
	fs.Parse(args)

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	mgr, err := wifi.New(ctx)
	if err != nil {
		fatal(err)
	}
	if err := fn(ctx, mgr); err != nil {
		fatal(err)
	}
}

func runList(ctx context.Context, mgr wifi.Manager) error {
	networks, err := mgr.ListNetworks(ctx)
	if err != nil {
		return err
	}
	if len(networks) == 0 {
		fmt.Println("近隣ネットワークが見つかりませんでした")
		return nil
	}
	printNetworks(networks)
	return nil
}

func runCurrent(ctx context.Context, mgr wifi.Manager) error {
	network, err := mgr.CurrentNetwork(ctx)
	if err != nil {
		return err
	}
	if network == nil {
		fmt.Println("現在, どのネットワークにも接続していません")
		return nil
	}
	printNetworks([]wifi.Network{*network})
	return nil
}

func printNetworks(networks []wifi.Network) {
	w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
	fmt.Fprintln(w, "SSID\tSECURITY\tPHYMODE\tCHANNEL\tSIGNAL")
	for _, n := range networks {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", orDash(n.SSID), orDash(n.Security), orDash(n.PHYMode), orDash(n.Channel), signalString(n))
	}
	w.Flush()
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func signalString(n wifi.Network) string {
	switch {
	case n.SignalDBm != nil:
		return fmt.Sprintf("%d dBm", *n.SignalDBm)
	case n.SignalPercent != nil:
		return fmt.Sprintf("%d%%", *n.SignalPercent)
	default:
		return "-"
	}
}

func runConnect(args []string) {
	fs := flag.NewFlagSet("connect", flag.ExitOnError)
	timeout := fs.Duration("timeout", 20*time.Second, "操作のタイムアウト (system_profiler/shortcuts runは合わせて実測8秒程度かかるため余裕を持たせている)")
	fs.Parse(args)

	rest := fs.Args()
	if len(rest) < 1 {
		usage()
		os.Exit(2)
	}
	ssid := rest[0]
	var passphrase string
	if len(rest) > 1 {
		passphrase = rest[1]
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	mgr, err := wifi.New(ctx)
	if err != nil {
		fatal(err)
	}
	if err := mgr.Connect(ctx, ssid, passphrase); err != nil {
		fatal(err)
	}
	fmt.Println("接続を要求しました")
}

func runWatch(args []string) {
	fs := flag.NewFlagSet("watch", flag.ExitOnError)
	fs.Parse(args)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	mgr, err := wifi.New(ctx)
	if err != nil {
		fatal(err)
	}

	fmt.Println("接続先の変化を監視しています... (^Cで終了)")
	for network := range wifi.WatchCurrentNetwork(ctx, mgr) {
		if network == nil {
			fmt.Println("切断されました")
			continue
		}
		fmt.Printf("接続: %s\n", network.SSID)
	}
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, "エラー:", err)
	os.Exit(1)
}
