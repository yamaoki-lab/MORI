package wifi

import (
	"errors"
	"testing"
)

func TestReadCurrentSSID(t *testing.T) {
	ssid, ok := readCurrentSSID(func(path string) ([]byte, error) {
		if path != currentSSIDFile {
			t.Fatalf("想定外のパス: %s", path)
		}
		return []byte("Test-AP_2.4GHz #1 (Room:B)\n"), nil
	})
	if !ok || ssid != "Test-AP_2.4GHz #1 (Room:B)" {
		t.Fatalf("SSID: 期待値 \"Test-AP_2.4GHz #1 (Room:B)\", 実際 %q (ok=%v)", ssid, ok)
	}
}

func TestReadCurrentSSID_FileNotFound(t *testing.T) {
	_, ok := readCurrentSSID(func(string) ([]byte, error) {
		return nil, errors.New("no such file")
	})
	if ok {
		t.Fatal("ファイルが無い場合はfalseになるはず (未接続, またはショートカット未セットアップ)")
	}
}

func TestReadCurrentSSID_Empty(t *testing.T) {
	_, ok := readCurrentSSID(func(string) ([]byte, error) {
		return []byte("   \n"), nil
	})
	if ok {
		t.Fatal("空文字列の場合はfalseになるはず")
	}
}
