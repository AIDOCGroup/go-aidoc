

package asm

import (
	"testing"

	"encoding/hex"
)

// 测试反汇编有效evm代码的说明
func TestInstructionIteratorValid(t *testing.T) {
	cnt := 0
	script, _ := hex.DecodeString("61000000")

	it := NewInstructionIterator(script)
	for it.Next() {
		cnt++
	}

	if err := it.Error(); err != nil {
		t.Errorf("预计2，但遇到错误 %v 而不是。", err)
	}
	if cnt != 2 {
		t.Errorf("预计2，但得到%v。", cnt)
	}
}

// 测试反汇编无效evm代码的说明
func TestInstructionIteratorInvalid(t *testing.T) {
	cnt := 0
	script, _ := hex.DecodeString("6100")

	it := NewInstructionIterator(script)
	for it.Next() {
		cnt++
	}

	if it.Error() == nil {
		t.Errorf("预计会出现错误，但会获得%v。", cnt)
	}
}

// 测试反汇编空evm代码的说明
func TestInstructionIteratorEmpty(t *testing.T) {
	cnt := 0
	script, _ := hex.DecodeString("")

	it := NewInstructionIterator(script)
	for it.Next() {
		cnt++
	}

	if err := it.Error(); err != nil {
		t.Errorf("预期为 0，但遇到错误 %v 而不是。", err)
	}
	if cnt != 0 {
		t.Errorf("预计为 0，但得到 %v。", cnt)
	}
}
