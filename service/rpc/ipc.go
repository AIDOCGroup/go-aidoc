package rpc

import (
	"context"
	"net"

	"github.com/aidoc/go-aidoc/service/p2p/netutil"
)
// ServeListener 接受l上的连接，为它们提供JSON-RPC。
func (srv *Server) ServeListener(l net.Listener) error {
	for {
		conn, err := l.Accept()
		if netutil.IsTemporaryError(err) {
			log_rpc.Error("RPC 接受错误",   err)
			continue
		} else if err != nil {
			return err
		}
		log_rpc.Trace("接受的连接", "地址", conn.RemoteAddr())
		go srv.ServeCodec(NewJSONCodec(conn), OptionMethodInvocation|OptionSubscriptions)
	}
}
// DialIPC 创建一个连接到给定端点的新IPC客户端。
// 在Unix上，它假定端点是unix套接字的完整路径，而Windows端点是命名管道的标识符。
//
// 上下文用于初始连接建立。
// 它不会影响与客户端的后续交互。
//
func DialIPC(ctx context.Context, endpoint string) (*Client, error) {
	return newClient(ctx, func(ctx context.Context) (net.Conn, error) {
		return newIPCConnection(ctx, endpoint)
	})
}
