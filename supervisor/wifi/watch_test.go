package wifi

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// fakeManager はwatch.goのオーケストレーションをexecなしで検証するためのテスト専用実装
type fakeManager struct {
	mu         sync.Mutex
	current    *Network
	currentErr error

	waitCh chan error // テスト側がここへ送るとWaitForChangeがその値で解除される
}

func (f *fakeManager) ListNetworks(ctx context.Context) ([]Network, error) { return nil, nil }

func (f *fakeManager) Connect(ctx context.Context, ssid, passphrase string) error { return nil }

func (f *fakeManager) CurrentNetwork(ctx context.Context) (*Network, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.current, f.currentErr
}

func (f *fakeManager) setCurrent(n *Network, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.current, f.currentErr = n, err
}

func (f *fakeManager) WaitForChange(ctx context.Context) error {
	select {
	case err := <-f.waitCh:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

func TestWatchCurrentNetwork(t *testing.T) {
	fm := &fakeManager{waitCh: make(chan error)}
	fm.setCurrent(&Network{SSID: "first"}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ch := WatchCurrentNetwork(ctx, fm)

	got := <-ch
	if got == nil || got.SSID != "first" {
		t.Fatalf("初回送出の値が違う: %+v", got)
	}

	// SSIDが変化していなければ送出されない
	fm.setCurrent(&Network{SSID: "first"}, nil)
	fm.waitCh <- nil
	select {
	case n := <-ch:
		t.Fatalf("変化していないのに送出された: %+v", n)
	case <-time.After(100 * time.Millisecond):
	}

	// CurrentNetworkの一時エラーはスキップされ, ループは継続する
	fm.setCurrent(nil, errors.New("一時的な失敗"))
	fm.waitCh <- nil
	select {
	case n := <-ch:
		t.Fatalf("エラー時に送出された: %+v", n)
	case <-time.After(100 * time.Millisecond):
	}

	// 実際にSSIDが変わったら送出される
	fm.setCurrent(&Network{SSID: "second"}, nil)
	fm.waitCh <- nil
	got = <-ch
	if got == nil || got.SSID != "second" {
		t.Fatalf("変化後の送出値が違う: %+v", got)
	}

	// 切断 (nil) もSSIDの変化として送出される
	fm.setCurrent(nil, nil)
	fm.waitCh <- nil
	got = <-ch
	if got != nil {
		t.Fatalf("切断時はnilが送出されるべき: %+v", got)
	}

	// ctxキャンセルでチャネルがcloseされる
	cancel()
	if _, ok := <-ch; ok {
		t.Fatalf("ctxキャンセル後もチャネルがcloseされていない")
	}
}
