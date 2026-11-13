//go:build js && wasm

// Command plaincadwasm exposes the initial planar kernel to a browser worker.
package main

import (
	"plainsolid/internal/plaincadbridge"
	"syscall/js"
)

func main() {
	bridge := plaincadbridge.New()
	call := js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return `{"ok":false,"error":{"code":"INVALID","message":"expected one JSON string"}}`
		}
		return bridge.Call(args[0].String())
	})
	js.Global().Set("plainSolidCall", call)
	select {}
}
