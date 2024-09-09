// Reference implementation of the classic "tenprint" program.  Hopefully I get
// it working with forthgo too!
package main

import (
	"fmt"
	"os"
	"time"

	"math/rand"

	"golang.org/x/term"
)

func main() {
	n := 0

	stdin := int(os.Stdin.Fd())
	baseDuration, _err := time.ParseDuration("100ms")
	if _err != nil {
		panic(_err)
	}
	for {
		width := 80
		if w, _, err := term.GetSize(stdin); err == nil {
			width = w
		}
		if n >= width {
			n = 0
			if term.IsTerminal(stdin) {
				d := time.Duration(0)
				for i := rand.Uint32() % 2; i > 0; i-- {
					d = d + baseDuration
				}
				time.Sleep(baseDuration)
			}
			fmt.Println()
		}
		n++
		c := '/'
		switch rand.Uint32() % 4 {
		case 0:
		case 1:
			c = '\\'
		case 2:
			c = ' '
		}
		fmt.Printf("%c", c)
	}
}
