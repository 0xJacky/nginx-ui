package pty

import (
	"errors"
	"fmt"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

var modernConPTY = sync.OnceValue(func() bool {
	return windows.NewLazySystemDLL("kernel32.dll").NewProc("ReleasePseudoConsole").Find() == nil
})

// The legacy ABI is published in microsoft/terminal/src/winconpty/winconpty.h.
// Microsoft recommends explicit cleanup for systems lacking ReleasePseudoConsole:
// https://github.com/microsoft/terminal/discussions/19112
// Never inspect this layout on newer systems, where HPCON may change.
type legacyPseudoConsole struct {
	signal, reference, process windows.Handle
}

func closePseudoConsole(console windows.Handle) error {
	if modernConPTY() {
		windows.ClosePseudoConsole(console)
		return nil
	}
	return closeLegacyPseudoConsole(console)
}

func closeLegacyPseudoConsole(console windows.Handle) error {
	pc := (*legacyPseudoConsole)(unsafe.Pointer(console))
	var result error
	// Closing the signal pipe requests shutdown; releasing the reference lets
	// conhost exit. Own all three handles, including the Process handle leaked
	// by the inbox ClosePseudoConsole on Windows Server 2022. Do not call that
	// API as well: closing an already-reused handle would affect another owner.
	for _, handle := range []windows.Handle{pc.signal, pc.reference, pc.process} {
		if handle != 0 && handle != windows.InvalidHandle {
			result = errors.Join(result, windows.CloseHandle(handle))
		}
	}
	// CreatePseudoConsole allocates HPCON on the process heap. The native
	// output pump keeps draining until EOF, independently of this deallocation.
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	heap, _, _ := kernel.NewProc("GetProcessHeap").Call()
	ok, _, err := kernel.NewProc("HeapFree").Call(heap, 0, uintptr(console))
	if ok == 0 {
		result = errors.Join(result, fmt.Errorf("free legacy pseudo console: %w", err))
	}
	return result
}
