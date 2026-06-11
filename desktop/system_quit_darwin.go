//go:build darwin

package main

/*
#cgo darwin LDFLAGS: -framework Cocoa
void installextendai-labSystemQuitHook(void);
*/
import "C"

import "sync"

var installSystemQuitHookOnce sync.Once

func installSystemQuitHook() {
	installSystemQuitHookOnce.Do(func() {
		C.installextendai-labSystemQuitHook()
	})
}

//export extendai-labMarkSystemQuit
func extendai-labMarkSystemQuit() {
	markSystemQuitRequested()
}
