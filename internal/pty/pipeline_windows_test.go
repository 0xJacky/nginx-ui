package pty

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/0xJacky/Nginx-UI/settings"
	"github.com/gorilla/websocket"
	"golang.org/x/sys/windows"
)

func TestWindowsTerminalInheritedCtrlCIgnore(t *testing.T) {
	if os.Args[len(os.Args)-1] == "terminal-ctrlc-child" {
		for _, command := range []string{"cmd.exe", "powershell.exe"} {
			t.Run(command, func(t *testing.T) { testWindowsTerminal(t, command, false) })
		}
		return
	}
	// CREATE_NEW_PROCESS_GROUP disables Ctrl+C in the child, reproducing
	// launchers such as MSYS without changing this test process's handlers.
	ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
	defer cancel()
	child := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestWindowsTerminalInheritedCtrlCIgnore$", "-test.v", "-test.timeout=70s", "terminal-ctrlc-child")
	child.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP | windows.CREATE_NO_WINDOW}
	output, err := child.CombinedOutput()
	if err != nil {
		t.Fatalf("terminal inherited Ctrl+C ignore: %v\n%s", err, output)
	}
	t.Logf("cmd.exe and PowerShell interrupted successfully beneath a Ctrl+C-ignoring launcher:\n%s", output)
}

func TestWindowsTerminalPipeHandles(t *testing.T) {
	read, write, err := terminalPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer read.Close()
	defer write.Close()
	for _, file := range []*os.File{read, write} {
		var flags uint32
		ok, _, err := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation").Call(file.Fd(), uintptr(unsafe.Pointer(&flags)))
		if ok == 0 {
			t.Fatal(err)
		}
		if flags&windows.HANDLE_FLAG_INHERIT != 0 {
			t.Fatal("terminal pipe can be inherited by an unrelated child")
		}
	}
}

func TestWindowsTerminalLegacyConsoleCleanup(t *testing.T) {
	// Newer Windows may change HPCON's layout. Only the old platform's CI
	// exercises the actual legacy ABI; never force it on a modern host.
	if modernConPTY() {
		t.Skip("real legacy HPCON test requires ReleasePseudoConsole to be absent")
	}
	inputRead, inputWrite, err := terminalPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer inputWrite.Close()
	defer inputRead.Close()
	outputRead, outputWrite, err := terminalPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer outputRead.Close()
	defer outputWrite.Close()
	var console windows.Handle
	if err := windows.CreatePseudoConsole(windows.Coord{X: 90, Y: 60}, windows.Handle(inputRead.Fd()), windows.Handle(outputWrite.Fd()), 0, &console); err != nil {
		t.Fatal(err)
	}
	pc := *(*legacyPseudoConsole)(unsafe.Pointer(console))
	var observedProcess windows.Handle
	if err := windows.DuplicateHandle(windows.CurrentProcess(), pc.process, windows.CurrentProcess(), &observedProcess, windows.SYNCHRONIZE, false, 0); err != nil {
		_ = closeLegacyPseudoConsole(console)
		t.Fatal(err)
	}
	defer windows.CloseHandle(observedProcess)
	inputRead.Close()
	outputWrite.Close()
	drained := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, outputRead); close(drained) }()
	if err := closeLegacyPseudoConsole(console); err != nil {
		t.Fatal(err)
	}
	// None of the original handles may survive compatibility teardown.
	getInfo := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetHandleInformation")
	for _, h := range []windows.Handle{pc.signal, pc.reference, pc.process} {
		var flags uint32
		if ok, _, _ := getInfo.Call(uintptr(h), uintptr(unsafe.Pointer(&flags))); ok != 0 {
			t.Error("legacy ConPTY handle survived close")
		}
	}
	if status, err := windows.WaitForSingleObject(observedProcess, 5000); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("conhost survived close: status=%d err=%v", status, err)
	}
	select {
	case <-drained:
	case <-time.After(5 * time.Second):
		t.Fatal("legacy output drain did not finish")
	}
	t.Log("Legacy close released all three native handles and conhost exited")
}

func TestWindowsTerminalLegacyCleanupOwnership(t *testing.T) {
	// Modern systems exercise ownership with a synthetic, process-heap object,
	// never with an opaque HPCON returned by the modern OS.
	kernel := windows.NewLazySystemDLL("kernel32.dll")
	heap, _, _ := kernel.NewProc("GetProcessHeap").Call()
	getCount := kernel.NewProc("GetProcessHandleCount")
	count := func() uint32 {
		var n uint32
		if ok, _, err := getCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n))); ok == 0 {
			t.Fatal(err)
		}
		return n
	}
	before := count()
	signal, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	reference, err := windows.CreateEvent(nil, 0, 0, nil)
	if err != nil {
		windows.CloseHandle(signal)
		t.Fatal(err)
	}
	var process windows.Handle
	if err := windows.DuplicateHandle(windows.CurrentProcess(), windows.CurrentProcess(), windows.CurrentProcess(), &process, windows.SYNCHRONIZE, false, 0); err != nil {
		windows.CloseHandle(signal)
		windows.CloseHandle(reference)
		t.Fatal(err)
	}
	memory, _, err := kernel.NewProc("HeapAlloc").Call(heap, 0, unsafe.Sizeof(legacyPseudoConsole{}))
	if memory == 0 {
		windows.CloseHandle(signal)
		windows.CloseHandle(reference)
		windows.CloseHandle(process)
		t.Fatal(err)
	}
	*(*legacyPseudoConsole)(unsafe.Pointer(memory)) = legacyPseudoConsole{signal, reference, process}
	if err := closeLegacyPseudoConsole(windows.Handle(memory)); err != nil {
		t.Fatal(err)
	}
	if after := count(); after != before {
		t.Fatalf("synthetic legacy cleanup retained handles: %d -> %d", before, after)
	}
	t.Log("Synthetic legacy allocation released both event handles and the duplicated Process handle")
}

func TestWindowsTerminalShellExitWithoutReader(t *testing.T) {
	for _, command := range []string{"cmd.exe", "powershell.exe"} {
		t.Run(command, func(t *testing.T) {
			if command == "powershell.exe" {
				t.Setenv("PSModulePath", filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "Modules"))
			}
			terminal, err := startTerminal(command)
			if err != nil {
				t.Fatal(err)
			}
			p := terminal.(*windowsTerminal)
			t.Cleanup(func() {
				// Release the reader even when testing a broken wait path.
				_ = p.reader.Close()
				if err := p.Close(); err != nil {
					t.Error(err)
				}
			})
			// Let the shell initialize, then stop reading entirely to model a
			// downstream pump blocked in a browser write during shell exit.
			ready := make(chan error, 1)
			go func() {
				var output strings.Builder
				buf := make([]byte, bufferSize)
				for !strings.Contains(output.String(), ">") {
					n, err := p.Read(buf)
					if err != nil {
						ready <- err
						return
					}
					output.Write(buf[:n])
				}
				ready <- nil
			}()
			select {
			case err := <-ready:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("shell did not reach its initial prompt")
			}
			if _, err := p.Write([]byte("exit\r")); err != nil {
				t.Fatal(err)
			}
			select {
			case <-p.waitDone:
			case <-time.After(10 * time.Second):
				t.Fatal("root-shell exit blocked on unread output")
			}
			select {
			case <-p.readDone:
			case <-time.After(5 * time.Second):
				t.Fatal("native output reader remained blocked after shell exit")
			}
			t.Log("Root shell and native output drain stopped after downstream stopped reading")
		})

	}
}

func TestWindowsTerminalReadAfterClose(t *testing.T) {
	p, err := startTerminal("cmd.exe")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if n, err := p.Read(make([]byte, 1)); n != 0 || err != io.EOF {
		t.Fatalf("owned teardown must read as EOF: n=%d err=%v", n, err)
	}
}

func TestWindowsTerminalInteractiveAndDisconnect(t *testing.T) {
	for _, command := range []string{"cmd.exe", "powershell.exe"} {
		t.Run(command, func(t *testing.T) {
			t.Run("disconnect", func(t *testing.T) { testWindowsTerminal(t, command, false) })
			t.Run("shell_exit", func(t *testing.T) { testWindowsTerminal(t, command, true) })
		})
	}
}

func TestWindowsTerminalConsoleSizeHelper(t *testing.T) {
	if os.Args[len(os.Args)-1] != "terminal-size-child" {
		return
	}
	name, err := windows.UTF16PtrFromString("CONOUT$")
	if err != nil {
		t.Fatal(err)
	}
	console, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE, nil, windows.OPEN_EXISTING, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(console)
	// Resize is queued on ConPTY's out-of-band signal pipe, independently of
	// shell command input. Poll the real attached console, never set its size.
	deadline := time.Now().Add(5 * time.Second)
	var info windows.ConsoleScreenBufferInfo
	for {
		if err := windows.GetConsoleScreenBufferInfo(console, &info); err != nil {
			t.Fatal(err)
		}
		cols, rows := info.Window.Right-info.Window.Left+1, info.Window.Bottom-info.Window.Top+1
		if cols == 120 && rows == 30 {
			fmt.Printf("CONSOLE_SIZE_%d_%d\n", cols, rows)
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("console resize not applied: window=%dx%d buffer=%dx%d", cols, rows, info.Size.X, info.Size.Y)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func testWindowsTerminal(t *testing.T, command string, exitShell bool) {
	if command == "powershell.exe" {
		// Use Windows PowerShell's modules, not modules injected by the host
		// running these tests (for example, a separate PowerShell 7 runtime).
		t.Setenv("PSModulePath", filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "Modules"))
	}
	original := settings.TerminalSettings.StartCmd
	settings.TerminalSettings.StartCmd = command
	t.Cleanup(func() { settings.TerminalSettings.StartCmd = original })
	started := make(chan error, 1)
	finished := make(chan struct{})
	processHandle := make(chan windows.Handle, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer close(finished)
		ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			started <- err
			return
		}
		defer ws.Close()
		pipeline, err := NewPipeLine(ws)
		started <- err
		if err != nil {
			return
		}
		defer pipeline.Close()
		native := pipeline.(*Pipeline).Pty.(*windowsTerminal)
		var handle windows.Handle
		if err := windows.DuplicateHandle(windows.CurrentProcess(), native.process, windows.CurrentProcess(), &handle, windows.SYNCHRONIZE, false, 0); err != nil {
			t.Error(err)
			return
		}
		processHandle <- handle
		errors := make(chan error, 1)
		var pumps sync.WaitGroup
		pumps.Add(2)
		go func() { defer pumps.Done(); pipeline.ReadPtyAndWriteWs(errors) }()
		go func() { defer pumps.Done(); pipeline.ReadWsAndWritePty(errors) }()
		<-errors
		pipeline.Close()
		pumps.Wait()
	}))
	defer server.Close()
	ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	if err = <-started; err != nil {
		t.Fatal(err)
	}
	var handle windows.Handle
	select {
	case handle = <-processHandle:
	case <-time.After(5 * time.Second):
		t.Fatal("no shell process handle")
	}
	defer windows.CloseHandle(handle)
	ws.SetReadDeadline(time.Now().Add(30 * time.Second))
	input := "echo NGINX_UI_%OS%\r"
	if command == "powershell.exe" {
		input = "Write-Output ('NGINX_UI_' + $env:OS)\r"
	}
	if err = ws.WriteJSON(map[string]any{"Type": TypeData, "Data": input}); err != nil {
		t.Fatal(err)
	}
	var output strings.Builder
	for !strings.Contains(output.String(), "NGINX_UI_Windows_NT") {
		_, data, readErr := ws.ReadMessage()
		if readErr != nil {
			t.Fatalf("command output: %v; received %q", readErr, output.String())
		}
		output.Write(data)
	}
	t.Logf("Real %s expanded OS through the WebSocket; console session is active", command)
	// On Server 2022 a resize sent before the first console client finishes
	// attaching can be superseded by its initial 90x60 buffer. Synchronize on
	// a real command result before testing resize of the active session.
	if err = ws.WriteJSON(map[string]any{"Type": TypeResize, "Data": map[string]int{"Cols": 120, "Rows": 30}}); err != nil {
		t.Fatal(err)
	}
	// Run an independent console-API observer inside each shell's session.
	// RawUI is a PowerShell host abstraction, not the ConPTY API boundary.
	query := fmt.Sprintf("\"%s\" -test.run=^TestWindowsTerminalConsoleSizeHelper$ -test.v terminal-size-child\r", os.Args[0])
	if command == "powershell.exe" {
		query = fmt.Sprintf("Write-Output ('RAWUI_' + $Host.UI.RawUI.WindowSize.Width + '_' + $Host.UI.RawUI.WindowSize.Height); & '%s' '-test.run=^TestWindowsTerminalConsoleSizeHelper$' '-test.v' terminal-size-child\r", strings.ReplaceAll(os.Args[0], "'", "''"))
	}
	if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": query}); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	for !strings.Contains(output.String(), "CONSOLE_SIZE_120_30") {
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("native console resize not observed: %v; output %q", err, output.String())
		}
		output.Write(data)
	}
	t.Logf("%s: attached CONOUT$ reported actual 120x30 console dimensions (RawUI 90x60 observed: %t)", command, strings.Contains(output.String(), "RAWUI_90_60"))
	if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": "ping -t 127.0.0.1\r"}); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	for !strings.Contains(output.String(), "TTL=") {
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("child output: %v", err)
		}
		output.Write(data)
	}
	if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": "\x03"}); err != nil {
		t.Fatal(err)
	}
	output.Reset()
	for !strings.Contains(output.String(), ">") {
		_, data, err := ws.ReadMessage()
		if err != nil {
			t.Fatalf("Ctrl+C did not return to shell: %v; output %q", err, output.String())
		}
		output.Write(data)
	}
	t.Log("Ctrl+C stopped the real child command and returned to the shell")
	if exitShell {
		if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": "exit\r"}); err != nil {
			t.Fatal(err)
		}
		// Keep consuming output while the shell exits.
		go func() {
			for {
				if _, _, err := ws.ReadMessage(); err != nil {
					return
				}
			}
		}()
	} else {
		ws.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
		ws.Close()
	}
	select {
	case <-finished:
		t.Log("Terminal and both forwarding goroutines stopped")
	case <-time.After(10 * time.Second):
		t.Fatal("terminal leaked after normal WebSocket close")
	}
	if status, err := windows.WaitForSingleObject(handle, 2000); err != nil || status != windows.WAIT_OBJECT_0 {
		t.Fatalf("shell process still alive: status=%d err=%v", status, err)
	}
}

// The helper deliberately detaches a descendant from the terminal console.
// Closing HPCON alone cannot own the lifetime of such a process tree.
func TestWindowsTerminalHelperProcess(t *testing.T) {
	if len(os.Args) < 3 || os.Args[len(os.Args)-2] != "pty-tree-helper" {
		return
	}
	file := os.Args[len(os.Args)-1]
	if strings.HasSuffix(file, ".grandchild") {
		for {
			time.Sleep(time.Second)
		}
	}
	child := exec.Command(os.Args[0], "-test.run=^TestWindowsTerminalHelperProcess$", "pty-tree-helper", file+".grandchild")
	child.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	if err := child.Start(); err != nil {
		os.Exit(2)
	}
	data, _ := json.Marshal([]int{os.Getpid(), child.Process.Pid})
	if err := os.WriteFile(file, data, 0600); err != nil {
		os.Exit(3)
	}
	for {
		time.Sleep(time.Second)
	}
}

func TestWindowsTerminalProcessTree(t *testing.T) {
	for _, mode := range []string{"disconnect", "shell_exit", "concurrent_close"} {
		t.Run(mode, func(t *testing.T) {
			original := settings.TerminalSettings.StartCmd
			settings.TerminalSettings.StartCmd = "cmd.exe"
			t.Cleanup(func() { settings.TerminalSettings.StartCmd = original })
			ready := make(chan *Pipeline, 1)
			finished := make(chan struct{})
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				defer close(finished)
				ws, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
				if err != nil {
					t.Error(err)
					return
				}
				defer ws.Close()
				p, err := NewPipeLine(ws)
				if err != nil {
					t.Error(err)
					return
				}
				defer p.Close()
				ready <- p.(*Pipeline)
				errors := make(chan error, 1)
				var pumps sync.WaitGroup
				pumps.Add(2)
				go func() { defer pumps.Done(); p.ReadPtyAndWriteWs(errors) }()
				go func() { defer pumps.Done(); p.ReadWsAndWritePty(errors) }()
				<-errors
				p.Close()
				pumps.Wait()
			}))
			defer server.Close()
			ws, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer ws.Close()
			var p *Pipeline
			select {
			case p = <-ready:
			case <-time.After(10 * time.Second):
				t.Fatal("no terminal")
			}
			go func() {
				for {
					if _, _, err := ws.ReadMessage(); err != nil {
						return
					}
				}
			}()
			file := filepath.Join(t.TempDir(), "tree.json")
			// START returns the prompt while the detached child owns a grandchild.
			input := fmt.Sprintf("start \"\" /b \"%s\" -test.run=^TestWindowsTerminalHelperProcess$ pty-tree-helper \"%s\"\r", os.Args[0], file)
			if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": input}); err != nil {
				t.Fatal(err)
			}
			var pids []int
			deadline := time.Now().Add(10 * time.Second)
			for time.Now().Before(deadline) {
				data, err := os.ReadFile(file)
				if err == nil && json.Unmarshal(data, &pids) == nil && len(pids) == 2 {
					break
				}
				time.Sleep(20 * time.Millisecond)
			}
			if len(pids) != 2 {
				t.Fatal("helper did not start")
			}
			handles := make([]windows.Handle, 0, 2)
			for _, pid := range pids {
				h, err := windows.OpenProcess(windows.SYNCHRONIZE|windows.PROCESS_TERMINATE, false, uint32(pid))
				if err != nil {
					t.Fatal(err)
				}
				handles = append(handles, h)
				// Reap our test helpers even if the implementation under test leaks.
				t.Cleanup(func() { windows.TerminateProcess(h, 1); windows.WaitForSingleObject(h, 5000); windows.CloseHandle(h) })
				if status, _ := windows.WaitForSingleObject(h, 0); status != uint32(windows.WAIT_TIMEOUT) {
					t.Fatal("helper exited before disconnect")
				}
			}
			var closers sync.WaitGroup
			switch mode {
			case "disconnect":
				ws.Close()
			case "shell_exit":
				if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": "exit\r"}); err != nil {
					t.Fatal(err)
				}
			case "concurrent_close":
				if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": "exit\r"}); err != nil {
					t.Fatal(err)
				}
				for range 8 {
					closers.Add(1)
					go func() { defer closers.Done(); p.Close() }()
				}
				ws.Close()
			}
			select {
			case <-finished:
			case <-time.After(10 * time.Second):
				t.Fatal("WebSocket pumps/terminal did not stop")
			}
			closers.Wait()
			for i, h := range handles {
				if status, err := windows.WaitForSingleObject(h, 3000); err != nil || status != windows.WAIT_OBJECT_0 {
					t.Errorf("descendant %d survived: status=%d err=%v", pids[i], status, err)
				}
			}
			t.Logf("%s: checked child and detached grandchild, both pumps and concurrent closers", mode)
		})
	}
}

func TestWindowsTerminalRepeatedLifecycle(t *testing.T) {
	// Keep CreateProcess on the same OS thread so per-thread Windows caches
	// are not mistaken for per-session leaks. Close/resize remain concurrent.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	getCount := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	handleCount := func() uint32 {
		var n uint32
		ok, _, err := getCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n)))
		if ok == 0 {
			t.Fatal(err)
		}
		return n
	}
	cycle := func(flood bool) {
		p, err := startTerminal("cmd.exe")
		if err != nil {
			t.Fatal(err)
		}
		if flood {
			if _, err := p.Write([]byte("for /L %i in (1,1,1000000) do @echo FILL_THE_OUTPUT_PIPE_%i\r")); err != nil {
				t.Fatal(err)
			}
			time.Sleep(25 * time.Millisecond)
		} else {
			go io.Copy(io.Discard, p)
		}
		done := make(chan struct{})
		go func() {
			var tasks sync.WaitGroup
			for range 8 {
				tasks.Add(1)
				go func() {
					defer tasks.Done()
					_ = p.Resize(120, 30)
					if err := p.Close(); err != nil {
						t.Error(err)
					}
				}()
			}
			tasks.Wait()
			close(done)
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatal("close blocked with concurrent resize/close or unread output")
		}
	}
	// Warm the same concurrent and backpressure paths before measuring.
	for i := 0; i < 10; i++ {
		cycle(i%2 == 0)
	}
	// Total handle count includes the Go scheduler's retained OS-thread handles.
	// Discard an initialization sample if new runtime threads were created,
	// but require a stable sample within three attempts; never raise the leak limit.
	for attempt := 0; attempt < 3; attempt++ {
		runtime.GC()
		before := handleCount()
		beforeG := runtime.NumGoroutine()
		beforeT, _ := runtime.ThreadCreateProfile(nil)
		beforeTypes := terminalHandleTypes(t)
		for i := 0; i < 40; i++ {
			cycle(i%2 == 0)
		}
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			runtime.GC()
			threads, _ := runtime.ThreadCreateProfile(nil)
			if threads != beforeT || (handleCount() <= before+2 && runtime.NumGoroutine() <= beforeG+2) {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		after, afterG := handleCount(), runtime.NumGoroutine()
		afterT, _ := runtime.ThreadCreateProfile(nil)
		if beforeT != afterT {
			t.Logf("runtime initialization sample %d: OS-thread records %d -> %d; handles %d -> %d; measure again", attempt+1, beforeT, afterT, before, after)
			continue
		}
		t.Logf("40 cycles (20 unread-output), 8 concurrent close/resize callers: handles %d -> %d, goroutines %d -> %d", before, after, beforeG, afterG)
		if after > before+2 {
			t.Logf("handle types before=%v after=%v", beforeTypes, terminalHandleTypes(t))
			t.Errorf("handle leak: %d -> %d", before, after)
		}
		if afterG > beforeG+2 {
			t.Errorf("goroutine leak: %d -> %d", beforeG, afterG)
		}
		return
	}
	t.Fatal("OS-thread count never stabilized; unable to establish a leak-free lifecycle")
}

func TestWindowsTerminalFailedStartCleanup(t *testing.T) {
	// Keep failure cleanup separate from the concurrent-close stress test:
	// Windows initializes thread-local state when CreateProcess rejects an image.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	invalid := filepath.Join(t.TempDir(), "invalid.exe")
	if err := os.WriteFile(invalid, []byte("not a Windows executable"), 0600); err != nil {
		t.Fatal(err)
	}
	getCount := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetProcessHandleCount")
	handleCount := func() uint32 {
		var n uint32
		ok, _, err := getCount.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&n)))
		if ok == 0 {
			t.Fatal(err)
		}
		return n
	}
	failStart := func() {
		if p, err := startTerminal(invalid); err == nil {
			p.Close()
			t.Fatal("invalid executable started")
		}
	}
	for range 40 {
		failStart()
	}
	for attempt := 0; attempt < 3; attempt++ {
		runtime.GC()
		before := handleCount()
		beforeT, _ := runtime.ThreadCreateProfile(nil)
		beforeTypes := terminalHandleTypes(t)
		for range 40 {
			failStart()
		}
		// Teardown can complete asynchronously on current Windows. Require
		// eventual zero growth instead of sampling retained process handles
		// before ConPTY has finished exiting.
		deadline := time.Now().Add(5 * time.Second)
		for time.Now().Before(deadline) {
			runtime.GC()
			threads, _ := runtime.ThreadCreateProfile(nil)
			if threads != beforeT || handleCount() <= before {
				break
			}
			time.Sleep(25 * time.Millisecond)
		}
		after := handleCount()
		afterT, _ := runtime.ThreadCreateProfile(nil)
		if beforeT != afterT {
			t.Logf("failed-start initialization sample %d: OS-thread records %d -> %d; handles %d -> %d; measure again", attempt+1, beforeT, afterT, before, after)
			continue
		}
		t.Logf("40 failed starts after warmup: handles %d -> %d", before, after)
		if after > before {
			t.Logf("failed-start handle types before=%v after=%v", beforeTypes, terminalHandleTypes(t))
			t.Errorf("failed-start handle leak: %d -> %d", before, after)
		}
		return
	}
	t.Fatal("OS-thread count never stabilized during failed-start cleanup")
}

// Diagnose remote-only leaks without exposing object names or filesystem paths.
func terminalHandleTypes(t *testing.T) map[string]int {
	t.Helper()
	type entry struct {
		Handle                             windows.Handle
		HandleCount, PointerCount          uintptr
		Access, Type, Attributes, Reserved uint32
	}
	buf := make([]byte, 1<<20)
	var needed uint32
	if err := windows.NtQueryInformationProcess(windows.CurrentProcess(), windows.ProcessHandleInformation, unsafe.Pointer(&buf[0]), uint32(len(buf)), &needed); err != nil {
		t.Logf("handle type diagnostic unavailable: %v", err)
		return nil
	}
	count := *(*uintptr)(unsafe.Pointer(&buf[0]))
	offset := 2 * unsafe.Sizeof(uintptr(0))
	if count > (uintptr(len(buf))-offset)/unsafe.Sizeof(entry{}) {
		t.Fatal("invalid handle snapshot size")
	}
	entries := unsafe.Slice((*entry)(unsafe.Pointer(&buf[offset])), int(count))
	query := windows.NewLazySystemDLL("ntdll.dll").NewProc("NtQueryObject")
	names := map[uint32]string{}
	result := map[string]int{}
	for _, h := range entries {
		name, ok := names[h.Type]
		if !ok {
			name = fmt.Sprintf("type-%d", h.Type)
			var object [1024]byte
			status, _, _ := query.Call(uintptr(h.Handle), 2, uintptr(unsafe.Pointer(&object[0])), uintptr(len(object)), uintptr(unsafe.Pointer(&needed)))
			if status == 0 {
				name = (*windows.NTUnicodeString)(unsafe.Pointer(&object[0])).String()
			}
			names[h.Type] = name
		}
		result[name]++
	}
	return result
}

func TestWindowsTerminalRejectsMissingCommand(t *testing.T) {
	if terminal, err := startTerminal("nginx-ui-test-nonexistent-shell.exe"); err == nil {
		terminal.Close()
		t.Fatal("missing shell unexpectedly started")
	}
}

func TestWindowsTerminalResizeAndClose(t *testing.T) {
	terminal, err := startTerminal("cmd.exe")
	if err != nil {
		t.Fatal(err)
	}
	defer terminal.Close()
	for _, size := range [][2]uint16{{0, 30}, {120, 0}, {32768, 30}, {120, 32768}} {
		if err := terminal.Resize(size[0], size[1]); err == nil {
			t.Fatalf("accepted invalid size %v", size)
		}
	}
	if err := terminal.Close(); err != nil {
		t.Fatal(err)
	}
	if err := terminal.Close(); err != nil {
		t.Fatalf("close must be idempotent: %v", err)
	}
	if err := terminal.Resize(120, 30); err == nil {
		t.Fatal("resized closed terminal")
	}
}
