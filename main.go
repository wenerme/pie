package main

import (
	"github.com/robertkrimen/otto"
	_ "github.com/robertkrimen/otto/underscore"
	"io"
	"strings"
	"github.com/chzyer/readline"
	"fmt"
	"github.com/wenerme/pie/gpio"
	"time"
	"os"
	"github.com/juju/errors"
	"io/ioutil"
	"sync"
)

var initScript = `
(function(global){

})(this)
`

func main() {
	vm := otto.New()
	vm.Set("exit", func(call otto.FunctionCall) otto.Value {
		if len(call.ArgumentList) == 0 {
			os.Exit(0)
		}else {
			v, e := call.Argument(0).ToInteger()
			if e != nil {
				panic(e)
			}
			os.Exit(int(v))
		}
		return otto.UndefinedValue()
	})

	vm.Set("run", func(fn string) otto.Value {
		b, e := ioutil.ReadFile(fn)
		if e != nil {
			panic(e)
		}
		v, e := vm.Eval(b)
		if e != nil {
			panic(e)
		}
		return v
	})

	setupTimeFunction(vm)

	vm.Set("os", &_os{})
	vm.Set("time", &_time{Second:time.Second})
	vm.Set("fmt", &_fmt{})
	gpio, e := gpio.OpenDefault()
	if e != nil {
		fmt.Fprintln(os.Stderr, "Gpio init failed caused by", e)
		os.Stderr.Sync()
	}else {
		vm.Set("gpio", gpio)
	}
	_, e = vm.Eval(initScript)
	if e != nil {
		panic(errors.Annotate(e, "Init code failed"))
	}
	RunWithPromptAndPrelude(vm, "Pie>", "Pie v0.1")

}

type _os struct{}
type _time struct {
	Second time.Duration
}
type _fmt struct{}

func (*_os) Exit(code int) {
	os.Exit(code)
}
func (*_os) Open(name string) (*os.File, error) {
	return os.Open(name)
}
func (*_time)Sleep(t int64) {
	time.Sleep(time.Duration(t))
}

func (*_fmt)Printf(format string, a ...interface{}) (n int, err error) {
	return fmt.Printf(format, a...)
}
func (*_fmt)Println(a ...interface{}) (n int, err error) {
	return fmt.Println(a...)
}
// RunWithPromptAndPrelude runs a REPL with the given prompt and prelude.
func RunWithPromptAndPrelude(vm *otto.Otto, prompt, prelude string) error {
	if prompt == "" {
		prompt = ">"
	}

	prompt = strings.Trim(prompt, " ")
	prompt += " "

	//rl, err := readline.New(prompt)
	rl, err := readline.NewEx(&readline.Config{
		Prompt:                 prompt,
		HistoryFile:            "/tmp/readline-pie",
		DisableAutoSaveHistory: true,
		HistoryLimit: 1000,
		//UniqueEditLine: true,
	})
	if err != nil {
		panic(err)
	}
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
	lastInterrupt := time.Now()
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
				now := time.Now()
				if now.Sub(lastInterrupt) > 1 * time.Second {
					fmt.Println("Use exit() or double Control-C to quit")
					lastInterrupt = now
					continue
				}else {
					break
				}
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
			rl.SaveHistory(strings.Join(d,""))
			d = nil

			func() {
				defer func() {
					if e := recover(); e != nil {
						switch e.(type) {
						case error:
							fmt.Println(errors.ErrorStack(e.(error)))
						default:
							fmt.Println(e)
						}
					}
				}()
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
			}()
		}

		rl.Refresh()
	}

	return rl.Close()
}

func setupTimeFunction(vm *otto.Otto) {
	timeout := make(map[int]chan <-struct{})
	interval := make(map[int]chan <-struct{})
	mutex := sync.Mutex{}
	parseArgs := func(call otto.FunctionCall) (func(), int64) {
		delay, e := call.Argument(1).ToInteger()
		if e != nil {
			panic(e)
		}

		if call.Argument(0).IsFunction() {
			return func() {
				call.Argument(0).Call(call.This, call.ArgumentList[2:])
			}, delay
		}else {
			code, e := call.Argument(0).ToString()
			if e != nil {
				panic(e)
			}
			return func() {
				vm.Eval(code)
			}, delay
		}
	}
	vm.Set("setTimeout", func(call otto.FunctionCall) otto.Value {
		//var timeoutID = window.setTimeout(func, [delay, param1, param2, ...]);
		//var timeoutID = window.setTimeout(code, [delay]);
		code, delay := parseArgs(call)
		timer := time.After(time.Duration(delay) * time.Millisecond)
		quit := make(chan struct{})
		var id int
		mutex.Lock()
		defer mutex.Unlock()
		for i := 0;; i++ {
			if _, ok := timeout[i]; !ok {
				id = i
				timeout[i] = quit
				break
			}
		}

		go func() {
			for {
				select {
				case <-timer:
					code()
				case <-quit:
					return
				}
			}
		}()
		v, e := otto.ToValue(id)
		if e != nil {
			panic(e)
		}
		return v
	})
	vm.Set("setInterval", func(call otto.FunctionCall) otto.Value {
		//var intervalID = window.setInterval(func, delay[, param1, param2, ...]);
		//var intervalID = window.setInterval(code, delay);
		code, delay := parseArgs(call)

		ticker := time.NewTicker(time.Duration(delay) * time.Millisecond)
		quit := make(chan struct{})
		var id int
		mutex.Lock()
		defer mutex.Unlock()
		for i := 0;; i++ {
			if _, ok := interval[i]; !ok {
				id = i
				interval[i] = quit
				break
			}
		}
		go func() {
			for {
				select {
				case <-ticker.C:
					code()
				case <-quit:
					ticker.Stop()
					return
				}
			}
		}()
		v, e := otto.ToValue(id)
		if e != nil {
			panic(e)
		}
		return v
	})
}
