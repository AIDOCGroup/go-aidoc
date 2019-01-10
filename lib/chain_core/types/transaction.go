package types

import (
	"errors"
	"io"
	"math/big"
	"sync/atomic"

	"github.com/aidoc/go-aidoc/lib/chain_common"
	"github.com/aidoc/go-aidoc/lib/hexutil"
	"github.com/aidoc/go-aidoc/lib/crypto"
	"github.com/aidoc/go-aidoc/lib/rlp"
	"github.com/aidoc/go-aidoc/lib/logger"
)

//go:generate gencodec -type txdata -field-override txdataMarshaling -out gen_tx_json.go

var (
	ErrInvalidSig = errors.New("invalid transaction v, r, s values")
)

// deriveSigner 对使用哪个签名者进行*最佳*猜测。
//获得签名
func deriveSigner(V *big.Int) Signer {
	if V.Sign() != 0 && isProtectedV(V) {
		return NewEIP155Signer(deriveChainId(V))
	} else {
		return HomesteadSigner{}
	}
}

//交易的数据结构定义
type Transaction struct {
	data txdata
	// caches
	hash atomic.Value
	size atomic.Value
	from atomic.Value
}

/**
 * 这里没有交易的发起者，因为发起者可以通过签名的数据获得
 */
type txdata struct {
	AccountNonce uint64                `json:"nonce"    gencodec:"required"` //AccountNonce 交易发送者已经发送交易的次数
	Price        *big.Int              `json:"gasPrice" gencodec:"required"` //此次交易的gas费用
	GasLimit     uint64                `json:"gas"      gencodec:"required"` //GasLimit 本次交易允许消耗gas的最大数量
	Recipient    *chain_common.Address `json:"to"       rlp:"nil"`           // nil 意味着合同创建  //交易的接收者
	Amount       *big.Int              `json:"value"    gencodec:"required"` //交易 AIDOC的数量
	Payload      []byte                `json:"input"    gencodec:"required"` //Payload是交易携带的数据

	// V，R，S是交易的签名数据
	// 签名值
	V *big.Int `json:"v" gencodec:"required"`
	R *big.Int `json:"r" gencodec:"required"`
	S *big.Int `json:"s" gencodec:"required"`

	// 这仅在编组到JSON时使用。
	Hash *chain_common.Hash `json:"hash" rlp:"-"`
}

type txdataMarshaling struct {
	AccountNonce hexutil.Uint64
	Price        *hexutil.Big
	GasLimit     hexutil.Uint64
	Amount       *hexutil.Big
	Payload      hexutil.Bytes
	V            *hexutil.Big
	R            *hexutil.Big
	S            *hexutil.Big
}


func NewTransaction(nonce uint64, to chain_common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newTransaction(nonce, &to, amount, gasLimit, gasPrice, data)
}

func NewContractCreation(nonce uint64, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	return newTransaction(nonce, nil, amount, gasLimit, gasPrice, data)
}

func newTransaction(nonce uint64, to *chain_common.Address, amount *big.Int, gasLimit uint64, gasPrice *big.Int, data []byte) *Transaction {
	if len(data) > 0 {
		data = chain_common.CopyBytes(data)
	}
	d := txdata{
		AccountNonce: nonce,
		Recipient:    to,
		Payload:      data,
		Amount:       new(big.Int),
		GasLimit:     gasLimit,
		Price:        new(big.Int),
		V:            new(big.Int),
		R:            new(big.Int),
		S:            new(big.Int),
	}
	if amount != nil {
		d.Amount.Set(amount)
	}
	if gasPrice != nil {
		d.Price.Set(gasPrice)
	}

	return &Transaction{data: d}
}
// ChainId返回此交易签名的链ID（如果有的话）
func (tx *Transaction) ChainId() *big.Int {
	return deriveChainId(tx.data.V)
}
//受保护的返回是否保护交易不受重播保护。
func (tx *Transaction) Protected() bool {
	logger.Info("transaction.go Protected()" , "tx.data.V" , tx.data.V)
	return isProtectedV(tx.data.V)
}

func isProtectedV(V *big.Int) bool {
	if V.BitLen() <= 8 {
		v := V.Uint64()
		return v != 27 && v != 28
	}
	// 任何不是 27或 28的东西都被认为是不受保护的
	return true
}
// EncodeRLP实现了rlp.Encoder
func (tx *Transaction) EncodeRLP(w io.Writer) error {
	return rlp.Encode(w, &tx.data)
}
// EncodeRLP 实现了rlp.Encoder
func (tx *Transaction) DecodeRLP(s *rlp.Stream) error {
	_, size, _ := s.Kind()
	err := s.Decode(&tx.data)
	if err == nil {
		tx.size.Store(chain_common.StorageSize(rlp.ListSize(size)))
	}

	return err
}
// MarshalJSON编码web3 RPC交易格式。
func (tx *Transaction) MarshalJSON() ([]byte, error) {
	hash := tx.Hash()
	data := tx.data
	data.Hash = &hash
	return data.MarshalJSON()
}
// UnmarshalJSON解码web3 RPC交易格式。
func (tx *Transaction) UnmarshalJSON(input []byte) error {
	var dec txdata
	if err := dec.UnmarshalJSON(input); err != nil {
		return err
	}
	var V byte
	if isProtectedV(dec.V) {
		chainID := deriveChainId(dec.V).Uint64()
		V = byte(dec.V.Uint64() - 35 - 2*chainID)
	} else {
		V = byte(dec.V.Uint64() - 27)
	}
	if !crypto.ValidateSignatureValues(V, dec.R, dec.S, false) {
		return ErrInvalidSig
	}
	*tx = Transaction{data: dec}
	return nil
}

func (tx *Transaction) Data() []byte       { return chain_common.CopyBytes(tx.data.Payload) }
func (tx *Transaction) Gas() uint64        { return tx.data.GasLimit }
func (tx *Transaction) GasPrice() *big.Int { return new(big.Int).Set(tx.data.Price) }
func (tx *Transaction) Value() *big.Int    { return new(big.Int).Set(tx.data.Amount) }
func (tx *Transaction) Nonce() uint64      { return tx.data.AccountNonce }
func (tx *Transaction) CheckNonce() bool   { return true }

// 返回交易的收件人地址。
// 如果交易是合同创建，则返回nil。
func (tx *Transaction) To() *chain_common.Address {
	if tx.data.Recipient == nil {
		return nil
	}
	to := *tx.data.Recipient
	return &to
}


/**
 * 交易的hash会首先从Transaction的缓存中读取hash，如果缓存中没有，则通过rlpHash来计算hash，并将hash放入到缓存中。
  交易的hash是通过Hash()方法获得的。
 */
// 哈希哈希值tx的RLP编码。
// 它唯一地标识交易。
func (tx *Transaction) Hash() chain_common.Hash {
	if hash := tx.hash.Load(); hash != nil {
		return hash.(chain_common.Hash)
	}
	v := rlpHash(tx)
	tx.hash.Store(v)
	return v
}
// Size返回交易的真实RLP编码存储大小，通过编码和返回，或返回预先缓存的值。
func (tx *Transaction) Size() chain_common.StorageSize {
	if size := tx.size.Load(); size != nil {
		return size.(chain_common.StorageSize)
	}
	c := writeCounter(0)
	rlp.Encode(&c, &tx.data)
	tx.size.Store(chain_common.StorageSize(c))
	return chain_common.StorageSize(c)
}

// AsMessage将交易作为core.Message返回。
//
// AsMessage要求签名者派生发件人。
//
// XXX将邮件重命名为不那么随意的内容？
func (tx *Transaction) AsMessage(s Signer) (Message, error) {
	msg := Message{
		nonce:      tx.data.AccountNonce,
		gasLimit:   tx.data.GasLimit,
		gasPrice:   new(big.Int).Set(tx.data.Price),
		to:         tx.data.Recipient,
		amount:     tx.data.Amount,
		data:       tx.data.Payload,
		checkNonce: true,
	}

	var err error
	msg.from, err = Sender(s, tx)
	return msg, err
}
// WithSignature返回具有给定签名的新交易。
// 此签名需要按照黄皮书（v + 27）中的说明进行格式化。
func (tx *Transaction) WithSignature(signer Signer, sig []byte) (*Transaction, error) {
	r, s, v, err := signer.SignatureValues(tx, sig)
	if err != nil {
		return nil, err
	}
	cpy := &Transaction{data: tx.data}
	cpy.data.R, cpy.data.S, cpy.data.V = r, s, v
	return cpy, nil
}
//成本回报金额+ gasprice * gaslimit。
func (tx *Transaction) Cost() *big.Int {
	total := new(big.Int).Mul(tx.data.Price, new(big.Int).SetUint64(tx.data.GasLimit))
	total.Add(total, tx.data.Amount)
	return total
}

func (tx *Transaction) RawSignatureValues() (*big.Int, *big.Int, *big.Int) {
	return tx.data.V, tx.data.R, tx.data.S
}
