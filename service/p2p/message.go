package p2p

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"sync/atomic"
	"time"

	"github.com/aidoc/go-aidoc/lib/event"
	"github.com/aidoc/go-aidoc/service/p2p/discover"
	"github.com/aidoc/go-aidoc/lib/rlp"
	"github.com/aidoc/go-aidoc/lib/i18"
)

// Msg定义p2p消息的结构。
//
// 请注意，由于在发送过程中消耗了Payload阅读器，因此只能发送一次Msg。 无法创建消息并发送任意次数。
// 如果要重用编码结构，请将有效负载编码为字节数组，并使用bytes.Reader创建单独的Msg作为每次发送的有效负载。
type Msg struct {
	Code       uint64
	Size       uint32 // 有效载荷的大小
	Payload    io.Reader
	ReceivedAt time.Time
}
// 解码将消息的RLP内容解析为给定值，该值必须是指针。
//
// 有关解码规则，请参阅包rlp。
func (msg Msg) Decode(val interface{}) error {
	s := rlp.NewStream(msg.Payload, uint64(msg.Size))
	if err := s.Decode(val); err != nil {
		return newPeerError(errInvalidMsg, "(code %x) (size %d) %v", msg.Code, msg.Size, err)
	}
	return nil
}

func (msg Msg) String() string {
	return i18.I18_print.Sprintf("msg #%v (%v bytes)", msg.Code, msg.Size)
}

// Discard将任何剩余的有效载荷数据读入黑洞。
func (msg Msg) Discard() error {
	_, err := io.Copy(ioutil.Discard, msg.Payload)
	return err
}

type MsgReader interface {
	ReadMsg() (Msg, error)
}

type MsgWriter interface {
	// WriteMsg发送消息。 它将阻塞，直到消息的Payload已被另一端消耗。
	//
	// 请注意，消息只能发送一次，因为它们的有效负载读取器已耗尽。
	WriteMsg(Msg) error
}

// MsgReadWriter 提供编码消息的读写。
// 实现应该确保可以从多个 goroutine 同时调用 ReadMsg 和 WriteMsg。
type MsgReadWriter interface {
	MsgReader
	MsgWriter
}
// 发送使用给定代码写入RLP编码的消息。 数据应编码为RLP列表。
func Send(w MsgWriter, msgcode uint64, data interface{}) error {
	size, r, err := rlp.EncodeToReader(data)
	//i18.I18_print.Println("msg-" , r)
	if err != nil {
		return err
	}
	return w.WriteMsg(Msg{Code: msgcode, Size: uint32(size), Payload: r})
}

//  SendItems使用给定的代码和数据元素写入RLP。对于如下调用：
//
//  SendItems（w，代码，e1，e2，e3）
//
// 消息有效负载将是包含项目的RLP列表：
func SendItems(w MsgWriter, msgcode uint64, elems ...interface{}) error {
	return Send(w, msgcode, elems)
}
// eofSignal 用 eof 信令包装读者。 当包装的阅读器返回错误或已读取计数字节时，eof 通道关闭。
type eofSignal struct {
	wrapped io.Reader
	count   uint32 // 剩下的字节数
	eof     chan<- struct{}
}
// 注意：当使用eofSignal检测是否已读取消息有效负载时，可能不会为零大小的消息调用Read。
func (r *eofSignal) Read(buf []byte) (int, error) {
	if r.count == 0 {
		if r.eof != nil {
			r.eof <- struct{}{}
			r.eof = nil
		}
		return 0, io.EOF
	}

	max := len(buf)
	if int(r.count) < len(buf) {
		max = int(r.count)
	}
	n, err := r.wrapped.Read(buf[:max])
	r.count -= uint32(n)
	if (err != nil || r.count == 0) && r.eof != nil {
		r.eof <- struct{}{} // 告诉Peer，msg已被消耗
		r.eof = nil
	}
	return n, err
}
// MsgPipe创建一个消息管道。
// 一端的读取与另一端的写入匹配。
// 管道是全双工的，两端都实现了MsgReadWriter。
func MsgPipe() (*MsgPipeRW, *MsgPipeRW) {
	var (
		c1, c2  = make(chan Msg), make(chan Msg)
		closing = make(chan struct{})
		closed  = new(int32)
		rw1     = &MsgPipeRW{c1, c2, closing, closed}
		rw2     = &MsgPipeRW{c2, c1, closing, closed}
	)
	return rw1, rw2
}
// 管道关闭后，管道操作返回ErrPipeClosed。
var ErrPipeClosed = errors.New("p2p：在已关闭的消息管道上读取或写入")

// MsgPipeRW 是 MsgReadWriter 管道的端点。
type MsgPipeRW struct {
	w       chan<- Msg
	r       <-chan Msg
	closing chan struct{}
	closed  *int32
}

// 写消息在管道上发送消息。
// 它会阻塞，直到接收器消耗了消息有效负载。
func (p *MsgPipeRW) WriteMsg(msg Msg) error {
	if atomic.LoadInt32(p.closed) == 0 {
		consumed := make(chan struct{}, 1)
		msg.Payload = &eofSignal{msg.Payload, msg.Size, consumed}
		select {
		case p.w <- msg:
			if msg.Size > 0 {
				// 等待有效负载读取或丢弃
				select {
				case <-consumed:
				case <-p.closing:
				}
			}
			return nil
		case <-p.closing:
		}
	}
	return ErrPipeClosed
}
// ReadMsg 返回管道另一端发送的消息。
func (p *MsgPipeRW) ReadMsg() (Msg, error) {
	if atomic.LoadInt32(p.closed) == 0 {
		select {
		case msg := <-p.r:
			return msg, nil
		case <-p.closing:
		}
	}
	return Msg{}, ErrPipeClosed
}
// 关闭取消区块管道两端的任何挂起的ReadMsg和WriteMsg调用。
// 他们将返回ErrPipeClosed。
// 关闭还会中断来自消息有效内容的任何读取。
func (p *MsgPipeRW) Close() error {
	if atomic.AddInt32(p.closed, 1) != 1 {
		// 别人已经关门了
		atomic.StoreInt32(p.closed, 1) // 避免溢出
		return nil
	}
	close(p.closing)
	return nil
}
// ExpectMsg从r读取消息并验证其代码和编码的RLP内容是否与提供的值匹配。
// 如果content为nil，则丢弃有效负载并且不进行验证。
func ExpectMsg(r MsgReader, code uint64, content interface{}) error {
	msg, err := r.ReadMsg()
	if err != nil {
		return err
	}
	if msg.Code != code {
		return fmt.Errorf(i18.I18_print.Sprintf("消息代码不匹配：获得 %d，预期 %d", msg.Code, code))
	}
	if content == nil {
		return msg.Discard()
	}
	contentEnc, err := rlp.EncodeToBytes(content)
	if err != nil {
		panic("内容编码错误： " + err.Error())
	}
	if int(msg.Size) != len(contentEnc) {
		return fmt.Errorf(i18.I18_print.Sprintf("消息大小不匹配：得 %d，想要 %d", msg.Size, len(contentEnc)))
	}
	actualContent, err := ioutil.ReadAll(msg.Payload)
	if err != nil {
		return err
	}
	if !bytes.Equal(actualContent, contentEnc) {
		return fmt.Errorf(i18.I18_print.Sprintf("消息有效负载不匹配：\n 得到：%x \n 想要：%x", actualContent, contentEnc))
	}
	return nil
}

// msgEventer 包装一个 MsgReadWriter，并在发送或接收消息时发送事件
type msgEventer struct {
	MsgReadWriter

	feed     *event.Feed
	peerID   discover.NodeID
	Protocol string
}

// newMsgEventer 返回一个 msgEventer，它将消息事件发送到给定的 feed
func newMsgEventer(rw MsgReadWriter, feed *event.Feed, peerID discover.NodeID, proto string) *msgEventer {
	return &msgEventer{
		MsgReadWriter: rw,
		feed:          feed,
		peerID:        peerID,
		Protocol:      proto,
	}
}
// ReadMsg 从底层 MsgReadWriter 读取消息并发出“消息接收”事件
func (ev *msgEventer) ReadMsg() (Msg, error) {
	msg, err := ev.MsgReadWriter.ReadMsg()
	if err != nil {
		return msg, err
	}
	ev.feed.Send(&PeerEvent{
		Type:     PeerEventTypeMsgRecv,
		Peer:     ev.peerID,
		Protocol: ev.Protocol,
		MsgCode:  &msg.Code,
		MsgSize:  &msg.Size,
	})
	return msg, nil
}
// WriteMsg将消息写入基础 MsgReadWriter 并发出“message sent”事件
func (ev *msgEventer) WriteMsg(msg Msg) error {
	err := ev.MsgReadWriter.WriteMsg(msg)
	if err != nil {
		return err
	}
	ev.feed.Send(&PeerEvent{
		Type:     PeerEventTypeMsgSend,
		Peer:     ev.peerID,
		Protocol: ev.Protocol,
		MsgCode:  &msg.Code,
		MsgSize:  &msg.Size,
	})
	return nil
}
// 如果实现io.Closer接口，则关闭基础 MsgReadWriter
func (ev *msgEventer) Close() error {
	if v, ok := ev.MsgReadWriter.(io.Closer); ok {
		return v.Close()
	}
	return nil
}
