package factory

import (
	"fmt"
	"log"

	"github.com/dop251/goja"
)

// PrintFactory returns a JS function that logs its arguments and returns the
// formatted string. prefix is prepended to all output.
func PrintFactory(vm *goja.Runtime, prefix string) func(call goja.FunctionCall) goja.Value {
	return func(call goja.FunctionCall) goja.Value {
		prep := prefix
		args := call.Arguments
		pass := make([]interface{}, len(args))
		for i := 0; i < len(args); i++ {
			prep += " %v"
			pass[i] = args[i]
		}
		out := fmt.Sprintf(prep, pass...)
		log.Print(out)
		return vm.ToValue(out)
	}
}
