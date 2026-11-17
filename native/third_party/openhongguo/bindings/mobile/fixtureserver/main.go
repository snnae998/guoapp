// Command fixtureserver 在本机回放 testdata/protocol 的样本，供 Android 和
// iOS 绑定的测试使用。它把监听地址打印到标准输出的第一行。
//
// 用法：go run ./bindings/mobile/fixtureserver [-addr 127.0.0.1:0]
package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/beilunyang/openhongguo/bindings/internal/fixtures"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:0", "listen address")
	dir := flag.String("dir", "testdata/protocol", "fixture directory")
	flag.Parse()
	ln, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("http://%s\n", ln.Addr())
	log.Fatal(http.Serve(ln, fixtures.Handler(*dir)))
}
