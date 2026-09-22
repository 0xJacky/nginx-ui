package pty

type MsgType int

const (
	MsgTypeInit MsgType = iota
	TypeData
	TypeResize
	TypePing
)

type Runner interface {
	// Each started pump sends one completion, including nil on normal EOF.
	// Callers must collect both results after closing the session.
	ReadPtyAndWriteWs(errorChan chan error)
	ReadWsAndWritePty(errorChan chan error)
	Close()
}
