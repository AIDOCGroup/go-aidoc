package rpc

import (
	"net/http"
	"net"
	"strings"
)

// virtualHostHandler 是一个验证传入请求的Host-header的处理程序。
// virtualHostHandler 可以防止不使用CORS标头的DNS重新绑定攻击，因为它们针对 RPC api 进行域内请求。
// 相反，我们可以在 Host-header 上看到使用了哪个域，并针对白名单进行验证。
type virtualHostHandler struct {
	vhosts map[string]struct{}
	next   http.Handler
}

// ServeHTTP 通过 HTTP 提供 JSON-RPC 请求，实现 http.Handler
func (h *virtualHostHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// 如果未设置 r.Host，我们可以继续提供服务，因为浏览器会设置 Host 标头
	if r.Host == "" {
		h.next.ServeHTTP(w, r)
		return
	}
	host, _, err := net.SplitHostPort(r.Host)
	if err != nil {
		// 无效（冒号过多）或未指定端口
		host = r.Host
	}
	if ipAddr := net.ParseIP(host); ipAddr != nil {
		// 这是一个IP地址，我们可以为此服务
		h.next.ServeHTTP(w, r)
		return

	}
	// 不是IP地址，而是主机名。 需要验证
	if _, exist := h.vhosts["*"]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	if _, exist := h.vhosts[host]; exist {
		h.next.ServeHTTP(w, r)
		return
	}
	http.Error(w, "指定的主机无效", http.StatusForbidden)
}

func newVHostHandler(vhosts []string, next http.Handler) http.Handler {
	vhostMap := make(map[string]struct{})
	for _, allowedHost := range vhosts {
		vhostMap[strings.ToLower(allowedHost)] = struct{}{}
	}
	return &virtualHostHandler{vhostMap, next}
}
