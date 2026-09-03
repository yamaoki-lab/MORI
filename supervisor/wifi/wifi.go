// Package wifi は, ドローンとのWi-Fi接続を管理する
// SSID一覧の取得・接続の要求・現在接続中AP取得の3操作をOS非依存のインターフェースとして
// 定義し, 実装はビルドタグでOSごとに分離する (macOS: 開発用, Linux: 実機の本番対象)
// ドローン自体がAP (Tello系プロトコル) であるため, 中継やキャプティブポータルは
// 扱わない. 単純なクライアント⇔AP関連付けを前提とする
package wifi

import "context"

// Network は, スキャンで見つかった (または現在接続中の) ネットワーク1件を表す
// SSIDはドローン機体のラベルと1:1で対応するため, ここでは一切加工しない (生のまま保持)
type Network struct {
	SSID     string
	PHYMode  string // ネゴシエート/対応する802.11規格の生表記 (例 "802.11a/n/ac/ax"). 未取得なら空文字列
	Security string // セキュリティ方式の生表記. 未取得なら空文字列
	Channel  string // 生表記のまま保持 (例 "40 (5GHz, 160MHz)"). 未取得なら空文字列

	// 信号強度: OSごとに単位が異なるため, フィールドを分けて単位を明示する
	// (dBm⇔%の変換は近似的なものしかなく, ここで勝手に正規化はしない)
	SignalDBm     *int // macOS由来 (dBm). 取得できない場合はnil
	SignalPercent *int // Linux由来 (NetworkManagerの0-100品質値). 取得できない場合はnil
}

// Manager は, Wi-Fi接続管理の3操作 (Issue #1) に加え, 変化検知用のWaitForChangeを表す
type Manager interface {
	// ListNetworks は, 現在見えている近隣ネットワークの一覧を返す
	ListNetworks(ctx context.Context) ([]Network, error)

	// Connect は, 指定SSIDへの接続を要求する (fire-and-return.
	// 関連付け完了の確認・待機はしない. 必要なら呼び出し側がCurrentNetworkで確認する)
	// passphraseは開放ネットワークの場合は空文字列
	Connect(ctx context.Context, ssid, passphrase string) error

	// CurrentNetwork は, 現在接続中のネットワークを返す
	// 未接続の場合は (nil, nil) を返す (エラーではない)
	CurrentNetwork(ctx context.Context) (*Network, error)

	// WaitForChange は, 接続状態が変化した可能性がある時点でブロックを解除する
	// OSのイベント通知機構を使い, 固定間隔のポーリングは行わない
	// 実際に何が変化したかはこの後CurrentNetworkを呼んで判定する (WaitForChange自体は
	// シグナルを返すのみ). ctxがキャンセルされた場合はctx.Err()を返す
	WaitForChange(ctx context.Context) error
}
