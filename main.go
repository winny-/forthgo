package main

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
)

var (
	ErrStackUnderflow = errors.New("stack underflow")
	ErrDivisionByZero = errors.New("division by 0")
	ErrUnknownWord    = errors.New("unknown word")
	ErrInvalidWord    = errors.New("invalid word")
)

const (
	WordTypeNumber   WordType = iota
	WordTypeFunction WordType = iota
)

type BaseWord struct{}
type Word interface {
	code() UserDefinedWordFunction
}

func (word *BaseWord) code() UserDefinedWordFunction {
	panic("Should not be called")
}

func (word NumberWord) code() UserDefinedWordFunction {
	return func(ctx *Context) error {
		return ctx.push(word.number)
	}
}

func (word UserDefinedWord) code() UserDefinedWordFunction {
	return word.function
}

func (word *BaseWord) doc() string {
	panic("Should not be called")
}

func (word NumberWord) doc() string {
	panic("Should not be called")
}

func (word UserDefinedWord) doc() string {
	return word.description
}

type WordType int

type NumberWord struct {
	*BaseWord
	number int
}
type UserDefinedWord struct {
	*BaseWord
	tag         string
	function    UserDefinedWordFunction
	description string
}

type UserDefinedWordFunction func(*Context) error

type Context struct {
	stack   []int
	words   map[string]UserDefinedWord
	state   State
	verbose bool
	scratch *UserDefinedWord
}

func (ctx *Context) defineWord(name, doc string, function UserDefinedWordFunction) UserDefinedWord {
	upperName := strings.ToUpper(name)
	word := UserDefinedWord{
		tag:         name,
		function:    function,
		description: doc,
	}
	ctx.words[upperName] = word
	return word
}

func (ctx *Context) pop() (int, error) {
	if len(ctx.stack) == 0 {
		return -1, ErrStackUnderflow
	}
	index := len(ctx.stack) - 1
	v := ctx.stack[index]
	ctx.stack = ctx.stack[:index]
	return v, nil
}

func (ctx *Context) push(v int) error {
	ctx.stack = append(ctx.stack, v)
	//fmt.Printf("Pushing %d - see %v\n", v, ctx.stack)
	return nil
}

type PrimitiveWordFunction func(*Context, []int) error

func (ctx *Context) definePrimitiveWord(name, doc string, numInputs int, function PrimitiveWordFunction) UserDefinedWord {
	return ctx.defineWord(
		name,
		doc,
		func(ctx *Context) error {
			inputs := make([]int, numInputs)
			for index := range inputs {
				value, err := ctx.pop()
				if err != nil {
					return err
				}
				inputs[len(inputs)-1-index] = value
			}
			return function(ctx, inputs)
		},
	)
}

type BinaryWordFunction = func(ctx *Context, a, b int) error

func (ctx *Context) defineBinaryWord(name, doc string, function BinaryWordFunction) UserDefinedWord {
	return ctx.definePrimitiveWord(name, doc, 2, func(ctx2 *Context, inputs []int) error {
		return function(ctx2, inputs[0], inputs[1])
	})
}

func (ctx *Context) defineDerivedWord(name, doc, code string) UserDefinedWord {
	save := ctx.scratch
	ctx.scratch = &UserDefinedWord{
		tag:         name,
		description: doc,
		function: func(ctx2 *Context) error {
			return nil
		},
	}
	for _, s := range strings.Split(code, " ") {
		w, err := ctx.parseWord(s)
		if err != nil {
			panic(err)
		}
		ctx.chain(w)
	}
	ret := ctx.scratch
	ctx.scratch = save
	return ctx.defineWord(ret.tag, ret.description, ret.code())
}

func bool2int(b bool) int {
	if b {
		return 1
	} else {
		return 0
	}
}

func int2bool(i int) bool {
	return i != 0
}

func makeContext() *Context {
	ctx := new(Context)
	ctx.words = make(map[string]UserDefinedWord)
	ctx.scratch = nil
	ctx.defineBinaryWord("<", "less than", func(ctx *Context, a, b int) error {
		return ctx.push(bool2int(a < b))
	})
	ctx.defineBinaryWord("and", "conjunction", func(ctx *Context, a, b int) error {
		return ctx.push(bool2int(int2bool(a) && int2bool(b)))
	})
	ctx.defineBinaryWord("or", "disjunction", func(ctx *Context, a, b int) error {
		return ctx.push(bool2int(int2bool(a) || int2bool(b)))
	})
	ctx.definePrimitiveWord("invert", "negate", 1, func(ctx *Context, inputs []int) error {
		return ctx.push(bool2int(inputs[0] == 0))
	})
	ctx.defineBinaryWord("=", "equality", func(ctx *Context, a, b int) error {
		return ctx.push(bool2int(a == b))
	})
	// TODO define <cond> IF <body> THEN
	ctx.defineBinaryWord("mod", "modulo", func(ctx *Context, a, b int) error {
		return ctx.push(a % b)
	})
	ctx.defineBinaryWord("-", "subtract two numbers", func(ctx *Context, a int, b int) error {
		return ctx.push(a - b)
	})
	ctx.defineBinaryWord("+", "add two numbers", func(ctx *Context, a, b int) error {
		return ctx.push(a + b)
	})
	ctx.defineBinaryWord("*", "multiply two numbers", func(ctx *Context, a, b int) error {
		return ctx.push(a * b)
	})
	ctx.defineBinaryWord("/", "divide two numbers", func(ctx *Context, a, b int) error {
		if a == 0 || b == 0 {
			return ErrDivisionByZero
		}
		return ctx.push(a / b)
	})
	ctx.definePrimitiveWord("drop", "discard the topmost value", 1, func(ctx *Context, inputs []int) error {
		if ctx.verbose {
			fmt.Println(inputs[0])
		}
		return nil
	})
	ctx.defineDerivedWord(".", "discard the topmost value", "drop")
	ctx.defineDerivedWord("true", "true value", "1")
	ctx.defineDerivedWord("false", "false value", "0")
	ctx.defineDerivedWord("=0", "is n == 0", "0 =")
	ctx.definePrimitiveWord("dup", "duplicate topmost value", 1, func(ctx *Context, inputs []int) error {
		if err := ctx.push(inputs[0]); err != nil {
			return err
		}
		return ctx.push(inputs[0])
	})
	ctx.defineBinaryWord("swap", "swap two twopmost values", func(ctx *Context, a, b int) error {
		if err := ctx.push(b); err != nil {
			return err
		}
		return ctx.push(a)
	})
	ctx.defineBinaryWord("over", "copies second item to top", func(ctx *Context, a, b int) error {
		if err := ctx.push(a); err != nil {
			return err
		}
		if err := ctx.push(b); err != nil {
			return err
		}
		return ctx.push(a)
	})
	ctx.definePrimitiveWord("rot", "a b c -- b c a", 3, func(ctx *Context, inputs []int) error {
		if err := ctx.push(inputs[1]); err != nil {
			return err
		}
		if err := ctx.push(inputs[2]); err != nil {
			return err
		}
		return ctx.push(inputs[0])
	})
	ctx.definePrimitiveWord("bye", "exit forthgo", 0, func(ctx *Context, inputs []int) error {
		ctx.state = Halt
		return nil
	})
	ctx.defineDerivedWord("2dup", "a b -- a b a b", "over over")
	ctx.defineDerivedWord("-rot", "a b c -- c a b", "rot rot")
	ctx.defineDerivedWord("<=", "less than equal", "2dup < -rot = or")
	ctx.defineDerivedWord("<>", "not equal", "= invert")
	ctx.defineDerivedWord(">", "greater than", "<= invert")
	ctx.defineDerivedWord(">=", "greater equal than", "< invert")
	ctx.definePrimitiveWord(".w", "show all known words", 0, func(ctx *Context, inputs []int) error {
		maxWordChars := 0
		for _, word := range ctx.words {
			wordLen := len(word.tag)
			if wordLen > maxWordChars {
				maxWordChars = wordLen
			}
		}
		for _, word := range ctx.words {
			fmt.Printf("%-*s%s\n", maxWordChars+8, word.tag, word.doc())
		}
		return nil
	})
	ctx.definePrimitiveWord(".v", "toggle verbose", 0, func(ctx *Context, inputs []int) error {
		ctx.verbose = !ctx.verbose
		if ctx.verbose {
			fmt.Println("Enabled verbose mode.")
		} else {
			fmt.Println("Disabled verbose mode.")
		}
		return nil
	})
	ctx.definePrimitiveWord(".s", "show the stack contents", 0, func(ctx *Context, inputs []int) error {
		top := len(ctx.stack) - 1
		for i := top; i >= 0; i-- {
			msg := ""
			if i == top {
				msg = "<-- TOP     (last in)"
			} else if i == 0 && msg == "" {
				msg = "<-- BOTTOM (first in)"
			}
			fmt.Printf("%d%24s\n", ctx.stack[i], msg)
		}
		return nil
	})
	ctx.state = Continue
	return ctx
}

type State int

const (
	Halt            State = iota
	Continue        State = iota
	Pause           State = iota
	ReadName        State = iota
	ReadBody        State = iota
	ReadDescription State = iota
)

// evalWord evaluates what to do with s.
// evalWord relies heavily on ctx.state
// Internally, evalWord is split into two steps:
// 1. Parse and handle the input s based on ctx.state into a compiled word.
// 2. Execute the word if ctx.state permits it (i.e. ctx.state == Continue) or store it (i.e. ctx.state == ReadBody && s == ";")
// evalWord returns any execution error it encountered.
func (ctx *Context) evalWord(s string) error {
	if ctx.scratch == nil {
		ctx.scratch = &UserDefinedWord{
			tag:         "",
			description: "",
			function:    func(ctx2 *Context) error { return nil },
		}
	}
	switch ctx.state {
	case Halt:
		return nil
	case Pause:
		ctx.state = Continue
		return nil
	case ReadName:
		if _, err := parseInt(s); err == nil {
			ctx.state = Continue
			return ErrInvalidWord
		}
		ctx.scratch.tag = s
		ctx.scratch.description = ""
		ctx.state = ReadBody
		return nil
	case ReadBody:
		if s == "(" {
			ctx.state = ReadDescription
			return nil
		} else if s == ";" {
			ctx.state = Continue
			ctx.defineWord(ctx.scratch.tag, ctx.scratch.doc(), ctx.scratch.code())
			ctx.scratch = nil
			return nil
		} else {
			w, err := ctx.parseWord(s)
			if err != nil {
				ctx.state = Continue
				return err
			}
			ctx.chain(w)
			return nil
		}
	case ReadDescription:
		if s == ")" {
			ctx.state = ReadBody
			return nil
		}
		if ctx.scratch.description == "" {
			ctx.scratch.description = s
		} else {
			ctx.scratch.description += " " + s
		}
		return nil
	case Continue:
		if s == ":" {
			ctx.state = ReadName
			ctx.scratch = nil
			return nil
		} else {
			w, err := ctx.parseWord(s)
			if err != nil {
				return err
			}
			return w.code()(ctx)
		}
	default:
		panic(fmt.Sprintf("Unknown State = %v", ctx.state))
	}
}

// Chain (sequence) ctx.scratch followed by w.
func (ctx *Context) chain(w Word) {
	f1, f2 := ctx.scratch.code(), w.code()
	ctx.scratch = &UserDefinedWord{
		tag:         ctx.scratch.tag,
		description: ctx.scratch.description,
		function: func(ctx *Context) error {
			if err := f1(ctx); err != nil {
				return err
			}
			return f2(ctx)
		},
	}
}

func (ctx *Context) parseWord(s string) (Word, error) {
	i, err := parseInt(s)
	if err == nil {
		w := NumberWord{
			number: i,
		}
		return w, nil
	}
	reference, ok := ctx.words[strings.ToUpper(s)]
	if ok {
		return reference, nil
	}
	return nil, ErrUnknownWord

}

func parseInt(s string) (int, error) {
	i, err := strconv.ParseInt(s, 10, 32)
	if err != nil {
		return -1, err
	}
	return int(i), nil
}

func (ctx *Context) prompt() string {
	switch ctx.state {
	case Continue:
		return "ok "
	case ReadBody, ReadName:
		return "..."
	case ReadDescription:
		return "(.."
	case Halt:
		return ""
	default:
		return "???"
	}
}

func main() {
	scanner := bufio.NewScanner(os.Stdin)
	ctx := makeContext()
	ctx.verbose = true
	fmt.Print(ctx.prompt() + " ")
	for ctx.state != Halt && scanner.Scan() {
		for _, s := range strings.Split(scanner.Text(), " ") {
			if err := ctx.evalWord(s); err != nil {
				fmt.Println(err)
				break
			}
		}
		if prompt := ctx.prompt(); prompt != "" {
			fmt.Print(prompt + " ")
		}
	}
	if err := scanner.Err(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
