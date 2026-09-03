package wifi

import "context"

// WatchCurrentNetwork は, 接続先(SSID)が変化した時にのみ新しい値を送出するチャネルを返す
// 固定間隔のポーリングは行わず, Manager.WaitForChangeによるOSイベント通知を使う
// 実際のプッシュ通知 (SSE等, console側) はこのチャネルを消費する側の責務であり,
// 本関数はOS通知の待受と変化検出までを担う (transportを持たない)
// ctxがキャンセルされるとチャネルはcloseされる
func WatchCurrentNetwork(ctx context.Context, m Manager) <-chan *Network {
	ch := make(chan *Network)
	go func() {
		defer close(ch)
		var last *Network
		emit := func(cur *Network) bool {
			if !networkChanged(last, cur) {
				return true
			}
			last = cur
			select {
			case ch <- cur:
				return true
			case <-ctx.Done():
				return false
			}
		}
		if cur, err := m.CurrentNetwork(ctx); err == nil {
			if !emit(cur) {
				return
			}
		}
		for {
			if err := m.WaitForChange(ctx); err != nil {
				return // ctxキャンセル, またはWaitForChange自体の致命的な失敗
			}
			cur, err := m.CurrentNetwork(ctx)
			if err != nil {
				continue // 一時的な失敗はスキップし, 次のWaitForChangeへ
			}
			if !emit(cur) {
				return
			}
		}
	}()
	return ch
}

// networkChanged は, SSIDの変化 (接続/切断/接続先切替) のみを見る
// (PHYMode/信号強度等の変動だけでは "変化" とみなさない)
func networkChanged(a, b *Network) bool {
	if (a == nil) != (b == nil) {
		return true
	}
	return a != nil && a.SSID != b.SSID
}
