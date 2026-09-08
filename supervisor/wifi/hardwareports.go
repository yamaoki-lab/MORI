package wifi

import (
	"bufio"
	"fmt"
	"strings"
)

// parseHardwarePorts は, `networksetup -listallhardwareports`の出力から
// "Wi-Fi"のDevice名 (例 "en0") を取り出す. 環境によって変わりうるため
// (ハードコード禁止), 起動時にこれで動的に検出する
func parseHardwarePorts(output []byte) (string, error) {
	scanner := bufio.NewScanner(strings.NewReader(string(output)))
	inWiFiStanza := false
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "Hardware Port: "):
			inWiFiStanza = strings.TrimPrefix(line, "Hardware Port: ") == "Wi-Fi"
		case inWiFiStanza && strings.HasPrefix(line, "Device: "):
			return strings.TrimPrefix(line, "Device: "), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("networksetup -listallhardwareports の出力の読み取りに失敗しました: %w", err)
	}
	return "", fmt.Errorf("Wi-Fiのハードウェアポートが見つかりませんでした")
}
