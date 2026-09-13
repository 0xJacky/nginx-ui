package pty

import (
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

func TestWindowsTerminalInteractiveAndDisconnect(t *testing.T) {
	for _, command := range []string{"cmd.exe", "powershell.exe"} {
		t.Run(command, func(t *testing.T) {
			t.Run("disconnect", func(t *testing.T) { testWindowsTerminal(t, command, false) })
			t.Run("shell_exit", func(t *testing.T) { testWindowsTerminal(t, command, true) })
		})
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
	if err = ws.WriteJSON(map[string]any{"Type": TypeResize, "Data": map[string]int{"Cols": 120, "Rows": 30}}); err != nil {
		t.Fatal(err)
	}
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
	t.Logf("Real %s expanded OS through the WebSocket after resize", command)
	if command == "powershell.exe" {
		if err := ws.WriteJSON(map[string]any{"Type": TypeData, "Data": "Write-Output ('SIZE_' + $Host.UI.RawUI.WindowSize.Width + '_' + $Host.UI.RawUI.WindowSize.Height)\r"}); err != nil {
			t.Fatal(err)
		}
		output.Reset()
		for !strings.Contains(output.String(), "SIZE_120_30") {
			_, data, err := ws.ReadMessage()
			if err != nil {
				t.Fatalf("resize not visible to shell: %v; output %q", err, output.String())
			}
			output.Write(data)
		}
		t.Log("PowerShell observed actual 120x30 console dimensions")
	}
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
		for range 40 {
			failStart()
		}
		runtime.GC()
		after := handleCount()
		afterT, _ := runtime.ThreadCreateProfile(nil)
		if beforeT != afterT {
			t.Logf("failed-start initialization sample %d: OS-thread records %d -> %d; handles %d -> %d; measure again", attempt+1, beforeT, afterT, before, after)
			continue
		}
		t.Logf("40 failed starts after warmup: handles %d -> %d", before, after)
		if after > before+2 {
			t.Errorf("failed-start handle leak: %d -> %d", before, after)
		}
		return
	}
	t.Fatal("OS-thread count never stabilized during failed-start cleanup")
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
