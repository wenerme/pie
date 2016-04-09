package main

import (
	"github.com/robertkrimen/otto"
	_ "github.com/robertkrimen/otto/underscore"
	"io"
	"strings"
	"github.com/chzyer/readline"
	"fmt"
	"github.com/wenerme/pie/gpio"
)

var script = `
/*
gpio
*/
`

func main() {
	vm := otto.New()
	vm.Set("sys", &sys{})
	gpio, e := gpio.OpenDefault()
	if e != nil {
		panic(e)
	}
	vm.Set("gpio", gpio)
	RunWithPromptAndPrelude(vm, "Pie>", "Pie v0.1")
}

type sys struct {

}

func (*sys) Check() bool {
	fmt.Println("OK")
	return true
}


// RunWithPromptAndPrelude runs a REPL with the given prompt and prelude.
func RunWithPromptAndPrelude(vm *otto.Otto, prompt, prelude string) error {
	if prompt == "" {
		prompt = ">"
	}

	prompt = strings.Trim(prompt, " ")
	prompt += " "

	rl, err := readline.New(prompt)
	if err != nil {
		return err
	}

	if prelude != "" {
		if _, err := io.Copy(rl.Stderr(), strings.NewReader(prelude + "\n")); err != nil {
			return err
		}

		rl.Refresh()
	}

	var d []string

	for {
		l, err := rl.Readline()
		if err != nil {
			if err == readline.ErrInterrupt {
				if d != nil {
					d = nil

					rl.SetPrompt(prompt)
					rl.Refresh()

					continue
				}
				break
			}
			return err
		}

		if l == "" {
			continue
		}

		d = append(d, l)

		s, err := vm.Compile("repl", strings.Join(d, "\n"))
		if err != nil {
			rl.SetPrompt(strings.Repeat(" ", len(prompt)))
		} else {
			rl.SetPrompt(prompt)

			d = nil

			v, err := vm.Eval(s)
			if err != nil {
				if oerr, ok := err.(*otto.Error); ok {
					io.Copy(rl.Stdout(), strings.NewReader(oerr.String()))
				} else {
					io.Copy(rl.Stdout(), strings.NewReader(err.Error()))
				}
			} else {
				rl.Stdout().Write([]byte(v.String() + "\n"))
			}
		}

		rl.Refresh()
	}

	return rl.Close()
}
