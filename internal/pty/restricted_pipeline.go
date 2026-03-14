package pty

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"

	"github.com/0xJacky/Nginx-UI/internal/helper"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
)

const (
	restrictedPrompt         = "demo> "
	restrictedWorkingDir     = "/"
	restrictedCommandTimeout = 3 * time.Second
)

var (
	errRestrictedCommandNotAllowed = errors.New("command is not allowed in demo mode")
	errRestrictedCommandTooLong    = errors.New("command is too long")
	errRestrictedCommandInvalid    = errors.New("command contains unsupported shell syntax")
)

type restrictedCommand struct {
	name string
	args []string
}

var restrictedCommandSpecs = map[string]restrictedCommand{
	"whoami":              {name: "whoami"},
	"id":                  {name: "id"},
	"hostname":            {name: "hostname"},
	"date":                {name: "date"},
	"uptime":              {name: "uptime"},
	"uname -a":            {name: "uname", args: []string{"-a"}},
	"free -h":             {name: "free", args: []string{"-h"}},
	"df -h":               {name: "df", args: []string{"-h"}},
	"ls":                  {name: "ls"},
	"ls -la":              {name: "ls", args: []string{"-la"}},
	"cat /etc/os-release": {name: "cat", args: []string{"/etc/os-release"}},
	"nginx -v":            {name: "nginx", args: []string{"-v"}},
	"nginx -V":            {name: "nginx", args: []string{"-V"}},
	"nginx -t":            {name: "nginx", args: []string{"-t"}},
}

var restrictedAllowedCommands = func() []string {
	commands := make([]string, 0, len(restrictedCommandSpecs)+3)
	commands = append(commands, "help", "clear", "pwd")
	for command := range restrictedCommandSpecs {
		commands = append(commands, command)
	}
	slices.Sort(commands)
	return commands
}()

type RestrictedPipeline struct {
	ws     *websocket.Conn
	done   chan struct{}
	buffer []rune
}

func NewRestrictedPipeline(conn *websocket.Conn) (Runner, error) {
	return &RestrictedPipeline{
		ws:   conn,
		done: make(chan struct{}),
	}, nil
}

func (p *RestrictedPipeline) ReadPtyAndWriteWs(errorChan chan error) {
	if err := p.writeString(restrictedBanner()); err != nil {
		if helper.IsUnexpectedWebsocketError(err) {
			errorChan <- errors.Wrap(err, "Error ReadPtyAndWriteWs websocket write")
		}
		return
	}

	<-p.done
}

func (p *RestrictedPipeline) ReadWsAndWritePty(errorChan chan error) {
	defer close(p.done)

	for {
		msgType, payload, err := p.ws.ReadMessage()
		if err != nil {
			if helper.IsUnexpectedWebsocketError(err) {
				errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty unexpected close")
			} else {
				errorChan <- nil
			}
			return
		}
		if msgType != websocket.TextMessage {
			errorChan <- errors.Errorf("Error ReadWsAndWritePty Invalid msgType: %v", msgType)
			return
		}

		var msg Message
		err = json.Unmarshal(payload, &msg)
		if err != nil {
			errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal")
			return
		}

		switch msg.Type {
		case TypeData:
			var data string
			err = json.Unmarshal(msg.Data, &data)
			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty json.Unmarshal msg.Data")
				return
			}

			err = p.handleInput(data)
			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadWsAndWritePty handle restricted input")
				return
			}
		case TypeResize:
			continue
		case TypePing:
			err = p.ws.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(time.Second))
			if err != nil {
				errorChan <- errors.Wrap(err, "Error ReadSktAndWritePty write pong")
				return
			}
		default:
			errorChan <- errors.Errorf("Error ReadWsAndWritePty unknown msg.Type %v", msg.Type)
			return
		}
	}
}

func (p *RestrictedPipeline) Close() {
	_ = p.ws.Close()
}

func (p *RestrictedPipeline) handleInput(data string) error {
	if strings.ContainsRune(data, '\x1b') {
		return nil
	}

	for _, r := range data {
		switch r {
		case '\r', '\n':
			commandLine := string(p.buffer)
			p.buffer = p.buffer[:0]

			output, err := executeRestrictedCommand(commandLine)
			if err != nil {
				output = err.Error() + "\r\n"
			}

			if err := p.writeString("\r\n" + output + restrictedPrompt); err != nil {
				return err
			}
		case '\b', '\x7f':
			if len(p.buffer) == 0 {
				continue
			}
			p.buffer = p.buffer[:len(p.buffer)-1]
			if err := p.writeString("\b \b"); err != nil {
				return err
			}
		case '\x03':
			p.buffer = p.buffer[:0]
			if err := p.writeString("^C\r\n" + restrictedPrompt); err != nil {
				return err
			}
		case '\t':
			continue
		default:
			if r < 32 || r == 127 {
				continue
			}
			p.buffer = append(p.buffer, r)
			if err := p.writeString(string(r)); err != nil {
				return err
			}
		}
	}

	return nil
}

func (p *RestrictedPipeline) writeString(value string) error {
	return p.ws.WriteMessage(websocket.TextMessage, []byte(value))
}

func restrictedBanner() string {
	return fmt.Sprintf("Demo terminal is running in restricted mode.\r\nAllowed commands: %s\r\n\r\n%s",
		strings.Join(restrictedAllowedCommands, ", "),
		restrictedPrompt,
	)
}

func executeRestrictedCommand(raw string) (string, error) {
	command, err := normalizeRestrictedCommand(raw)
	if err != nil {
		return "", err
	}
	if command == "" {
		return "", nil
	}

	switch command {
	case "help":
		return fmt.Sprintf("Allowed commands:\r\n- %s\r\n", strings.Join(restrictedAllowedCommands, "\r\n- ")), nil
	case "clear":
		return "\x1b[2J\x1b[H", nil
	case "pwd":
		return restrictedWorkingDir + "\r\n", nil
	}

	spec, ok := restrictedCommandSpecs[command]
	if !ok {
		return "", errRestrictedCommandNotAllowed
	}

	ctx, cancel := context.WithTimeout(context.Background(), restrictedCommandTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, spec.name, spec.args...)
	cmd.Dir = restrictedWorkingDir

	output, err := cmd.CombinedOutput()
	if ctx.Err() == context.DeadlineExceeded {
		return "", errors.New("command timed out")
	}

	if len(output) == 0 && err == nil {
		return "", nil
	}

	result := string(output)
	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}
	result = strings.ReplaceAll(result, "\n", "\r\n")

	if err != nil {
		return result, nil
	}

	return result, nil
}

func normalizeRestrictedCommand(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if len(trimmed) > 128 {
		return "", errRestrictedCommandTooLong
	}
	if strings.ContainsAny(trimmed, "|&;><`$()\\*?") {
		return "", errRestrictedCommandInvalid
	}
	if strings.Contains(trimmed, "..") {
		return "", errRestrictedCommandInvalid
	}
	if strings.ContainsRune(trimmed, '\n') || strings.ContainsRune(trimmed, '\r') {
		return "", errRestrictedCommandInvalid
	}

	return strings.Join(strings.Fields(trimmed), " "), nil
}
