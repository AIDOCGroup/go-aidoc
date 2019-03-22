package vm

import "errors"

var (
	ErrOutOfGas                 = errors.New("gas不足")
	ErrCodeStoreOutOfGas        = errors.New("智能合约创建代码存储数据gas不足")
	ErrDepth                    = errors.New("超出最大通话深度")
	ErrTraceLimitReached        = errors.New("日志的数量达到指定的限度")
	ErrInsufficientBalance      = errors.New("转账余额不足")
	ErrContractAddressCollision = errors.New("智能合约地址冲突")
)
