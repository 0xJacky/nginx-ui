package pty

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"unsafe"

	"golang.org/x/sys/windows"
)

type windowsTerminal struct {
	input, output *os.File
	console       windows.Handle
	process       windows.Handle
	job           windows.Handle
	jobErr        error
	waitErr       error
	mu            sync.Mutex
	closed        bool
	closeOnce     sync.Once
	closeErr      error
	waitDone      chan struct{}
}

func startTerminal(command string) (_ terminal, err error) {
	if err := windows.NewLazySystemDLL("kernel32.dll").NewProc("CreatePseudoConsole").Find(); err != nil {
		return nil, fmt.Errorf("Windows ConPTY requires Windows 10 version 1809 or later: %w", err)
	}
	// Resolve the executable exactly as exec.Command does, including paths with spaces.
	path, err := exec.LookPath(command)
	if err != nil {
		return nil, err
	}
	application, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	commandLine, err := windows.UTF16PtrFromString(windows.EscapeArg(path))
	if err != nil {
		return nil, err
	}
	inputRead, inputWrite, err := os.Pipe()
	if err != nil {
		return nil, err
	}
	defer inputRead.Close()
	outputRead, outputWrite, err := os.Pipe()
	if err != nil {
		inputWrite.Close()
		return nil, err
	}
	defer outputWrite.Close()
	p := &windowsTerminal{input: inputWrite, output: outputRead}
	defer func() {
		if err != nil {
			_ = p.Close()
		}
	}()
	p.job, err = windows.CreateJobObject(nil, nil)
	if err != nil {
		return nil, fmt.Errorf("create terminal process job: %w", err)
	}
	limits := windows.JOBOBJECT_EXTENDED_LIMIT_INFORMATION{}
	limits.BasicLimitInformation.LimitFlags = windows.JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE
	if _, err = windows.SetInformationJobObject(p.job, windows.JobObjectExtendedLimitInformation, uintptr(unsafe.Pointer(&limits)), uint32(unsafe.Sizeof(limits))); err != nil {
		return nil, fmt.Errorf("configure terminal process job: %w", err)
	}
	err = windows.CreatePseudoConsole(windows.Coord{X: 90, Y: 60}, windows.Handle(inputRead.Fd()), windows.Handle(outputWrite.Fd()), 0, &p.console)
	if err != nil {
		return nil, fmt.Errorf("create Windows pseudo console: %w", err)
	}
	attributes, err := windows.NewProcThreadAttributeList(1)
	if err != nil {
		return nil, err
	}
	defer attributes.Delete()
	// This attribute takes the opaque HPCON value, not a pointer to the handle.
	if err = attributes.Update(windows.PROC_THREAD_ATTRIBUTE_PSEUDOCONSOLE, unsafe.Pointer(p.console), unsafe.Sizeof(p.console)); err != nil {
		return nil, err
	}
	startup := windows.StartupInfoEx{ProcThreadAttributeList: attributes.List()}
	startup.Cb = uint32(unsafe.Sizeof(startup))
	// Do not reuse the nginx-ui host's standard handles when it runs in a console.
	startup.Flags = windows.STARTF_USESTDHANDLES
	var process windows.ProcessInformation
	// Assign the suspended shell before it can create any descendants. Console
	// detachment does not remove a descendant from this non-breakaway job.
	err = windows.CreateProcess(application, commandLine, nil, nil, false, windows.EXTENDED_STARTUPINFO_PRESENT|windows.CREATE_SUSPENDED, nil, nil, &startup.StartupInfo, &process)
	if err != nil {
		return nil, fmt.Errorf("start Windows terminal process: %w", err)
	}
	defer windows.CloseHandle(process.Thread)
	p.process = process.Process
	p.waitDone = make(chan struct{})
	go p.wait()
	if err = windows.AssignProcessToJobObject(p.job, p.process); err != nil {
		// The shell is still suspended and has no children or job ownership.
		_ = windows.TerminateProcess(p.process, 1)
		return nil, fmt.Errorf("assign terminal process job: %w", err)
	}
	if _, err = windows.ResumeThread(process.Thread); err != nil {
		return nil, fmt.Errorf("resume terminal process: %w", err)
	}
	return p, nil
}

func (p *windowsTerminal) wait() {
	defer close(p.waitDone)
	_, p.waitErr = windows.WaitForSingleObject(p.process, windows.INFINITE)
	p.mu.Lock()
	defer p.mu.Unlock()
	// Flush the final shell output, then let the reader observe EOF. Keeping
	// HPCON alive after the shell exits would leave the WebSocket open forever.
	p.closeConsole()
}

// The caller holds mu. Close closes the pipes before taking this lock so it
// can also unblock a flush when the browser stops consuming output.
func (p *windowsTerminal) closeConsole() {
	// Kill console-detached descendants as well, both on browser disconnect
	// and when the root shell exits normally. Only wait owns process waiting.
	if p.job != 0 {
		p.jobErr = windows.CloseHandle(p.job)
		p.job = 0
	}
	if p.console != 0 {
		windows.ClosePseudoConsole(p.console)
		p.console = 0
	}
	p.closed = true
}

func (p *windowsTerminal) Read(data []byte) (int, error)  { return p.output.Read(data) }
func (p *windowsTerminal) Write(data []byte) (int, error) { return p.input.Write(data) }

func (p *windowsTerminal) Resize(cols, rows uint16) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.closed {
		return os.ErrClosed
	}
	if cols == 0 || rows == 0 || cols > 32767 || rows > 32767 {
		return fmt.Errorf("invalid Windows terminal dimensions: %dx%d", cols, rows)
	}
	return windows.ResizePseudoConsole(p.console, windows.Coord{X: int16(cols), Y: int16(rows)})
}

func (p *windowsTerminal) Close() error {
	p.closeOnce.Do(func() {
		inputErr := p.input.Close()
		outputErr := p.output.Close()
		p.mu.Lock()
		p.closeConsole()
		p.mu.Unlock()
		if p.waitDone != nil {
			<-p.waitDone
		}
		var processErr error
		if p.process != 0 {
			processErr = windows.CloseHandle(p.process)
		}
		p.closeErr = errors.Join(inputErr, outputErr, processErr, p.jobErr, p.waitErr)
	})
	return p.closeErr
}
