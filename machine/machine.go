package machine

import (
	"net"
	"os"
	"sync"
)

var (
	once sync.Once
	info map[string]interface{}
)

// Info 返回机器信息（hostname + ip），惰性初始化。
// 用于注入到统一 Response 的 _pod 字段。
func Info() map[string]interface{} {
	once.Do(func() {
		info = map[string]interface{}{
			"hostname": "",
			"ip":       "",
		}
		if hostname, err := os.Hostname(); err == nil {
			info["hostname"] = hostname
		}
		// 通过 UDP dial 获取本机出口 IP（不实际发包）
		if conn, err := net.Dial("udp", "114.114.114.114:80"); err == nil {
			defer conn.Close()
			if addr, ok := conn.LocalAddr().(*net.UDPAddr); ok {
				info["ip"] = addr.IP.String()
			}
		}
	})
	return info
}
