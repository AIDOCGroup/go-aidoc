package asm

import (
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/aidoc/go-aidoc/lib/chain_core/vm"
	"github.com/aidoc/go-aidoc/lib/i18"
	"github.com/aidoc/go-aidoc/lib/math"
)

//编译器包含有关已解析源的信息并保存程序的标记。
type Compiler struct {
	tokens []token
	binary []interface{}

	labels map[string]int

	pc, pos int

	debug bool
}

// NewCompiler 返回一个新分配的编译器。
func NewCompiler(debug bool) *Compiler {
	return &Compiler{
		labels: make(map[string]int),
		debug:  debug,
	}
}

// 将令牌馈送到 ch 并由编译器解释。
//
// feed是编译阶段的第一次传递，因为它收集程序中使用过的标签并保留一个程序计数器，用于确定
// 跳转底座的位置。 可以在第二阶段使用标签来推动标签并确定正确的位置。
func (c *Compiler) Feed(ch <-chan token) {
	for i := range ch {
		switch i.typ {
		case number:
			num := math.MustParseBig256(i.text).Bytes()
			if len(num) == 0 {
				num = []byte{0}
			}
			c.pc += len(num)
		case stringValue:
			c.pc += len(i.text) - 2
		case element:
			c.pc++
		case labelDef:
			c.labels[i.text] = c.pc
			c.pc++
		case label:
			c.pc += 5
		}

		c.tokens = append(c.tokens, i)
	}
	if c.debug {
		fmt.Fprintln(os.Stderr, "found", len(c.labels), "labels")
	}
}

// 编译编译当前标记并返回可由EVM解释的二进制字符串，如果失败则返回错误。
//
// compile是编译阶段的第二个阶段，它将令牌编译为EVM指令。
func (c *Compiler) Compile() (string, []error) {
	var errors []error
	// 继续循环遍历令牌，直到堆栈耗尽。
	for c.pos < len(c.tokens) {
		if err := c.compileLine(); err != nil {
			errors = append(errors, err)
		}
	}

	// 将二进制转换为十六进制
	var bin string
	for _, v := range c.binary {
		switch v := v.(type) {
		case vm.OpCode:
			bin += i18.I18_print.Sprintf("%x", []byte{byte(v)})
		case []byte:
			bin += i18.I18_print.Sprintf("%x", v)
		}
	}
	return bin, errors
}

// next返回下一个标记并递增位置。
func (c *Compiler) next() token {
	token := c.tokens[c.pos]
	c.pos++
	return token
}

//编译行编译单行指令，例如 "push 1", "jump @label".。
func (c *Compiler) compileLine() error {
	n := c.next()
	if n.typ != lineStart {
		return compileErr(n, n.typ.String(), lineStart.String())
	}

	lvalue := c.next()
	switch lvalue.typ {
	case eof:
		return nil
	case element:
		if err := c.compileElement(lvalue); err != nil {
			return err
		}
	case labelDef:
		c.compileLabel()
	case lineEnd:
		return nil
	default:
		return compileErr(lvalue, lvalue.text, i18.I18_print.Sprintf("%v or %v", labelDef, element))
	}

	if n := c.next(); n.typ != lineEnd {
		return compileErr(n, n.text, lineEnd.String())
	}

	return nil
}

// compileNumber将数字编译为字节
func (c *Compiler) compileNumber(element token) (int, error) {
	num := math.MustParseBig256(element.text).Bytes()
	if len(num) == 0 {
		num = []byte{0}
	}
	c.pushBin(num)
	return len(num), nil
}

// compileElement将元素（push＆label或两者）编译为二进制表示，如果输入的语句不正确，则可能会出错。
func (c *Compiler) compileElement(element token) error {
	// 检查跳转 必须从右到左读取和编译跳转。
	if isJump(element.text) {
		rvalue := c.next()
		switch rvalue.typ {
		case number:
			// TODO 弄清楚如何正确地返回错误
			c.compileNumber(rvalue)
		case stringValue:
			// 引用字符串，删除它们。
			c.pushBin(rvalue.text[1 : len(rvalue.text)-2])
		case label:
			c.pushBin(vm.PUSH4)
			pos := big.NewInt(int64(c.labels[rvalue.text])).Bytes()
			pos = append(make([]byte, 4-len(pos)), pos...)
			c.pushBin(pos)
		default:
			return compileErr(rvalue, rvalue.text, "数字，字符串或标签")
		}
		// 推动操作
		c.pushBin(toBinary(element.text))
		return nil
	} else if isPush(element.text) {
		// 处理推。 从左到右 读操作 推送。
		var value []byte

		rvalue := c.next()
		switch rvalue.typ {
		case number:
			value = math.MustParseBig256(rvalue.text).Bytes()
			if len(value) == 0 {
				value = []byte{0}
			}
		case stringValue:
			value = []byte(rvalue.text[1 : len(rvalue.text)-1])
		case label:
			value = make([]byte, 4)
			copy(value, big.NewInt(int64(c.labels[rvalue.text])).Bytes())
		default:
			return compileErr(rvalue, rvalue.text, "数字，字符串或标签")
		}

		if len(value) > 32 {
			return fmt.Errorf(i18.I18_print.Sprintf("%d 类型错误: 不支持的字符串或数字的大小 > 32", rvalue.lineno))
		}

		c.pushBin(vm.OpCode(int(vm.PUSH1) - 1 + len(value)))
		c.pushBin(value)
	} else {
		c.pushBin(toBinary(element.text))
	}

	return nil
}

// compileLabel将跳转到二进制切片。
func (c *Compiler) compileLabel() {
	c.pushBin(vm.JUMPDEST)
}

// pushBin将值v推送到二进制堆栈。
func (c *Compiler) pushBin(v interface{}) {
	if c.debug {
		fmt.Printf("%d: %v\n", len(c.binary), v)
	}
	c.binary = append(c.binary, v)
}

// isPush返回字符串op是否为push（N）中的任何一个。
func isPush(op string) bool {
	return strings.ToUpper(op) == "PUSH"
}

// isJump返回字符串op是否为jump（i）
func isJump(op string) bool {
	return strings.ToUpper(op) == "JUMPI" || strings.ToUpper(op) == "JUMP"
}

// toBinary 将文本转换为 vm.OpCode
func toBinary(text string) vm.OpCode {
	return vm.StringToOp(strings.ToUpper(text))
}

type compileError struct {
	got  string
	want string

	lineno int
}

func (err compileError) Error() string {
	return i18.I18_print.Sprintf("%d 语法错误：意外 %v,预期 %v", err.lineno, err.got, err.want)
}

func compileErr(c token, got, want string) error {
	return compileError{
		got:    got,
		want:   want,
		lineno: c.lineno,
	}
}
