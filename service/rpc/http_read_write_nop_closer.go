package rpc

import "io"

// httpReadWriteNopCloser 使用 NOP Close 方法包装 io.Reader 和 io.Writer 。
type httpReadWriteNopCloser struct {
	io.Reader
	io.Writer
}

// 关闭什么都不做，总是返回零
func (t *httpReadWriteNopCloser) Close() error {
	return nil
}
