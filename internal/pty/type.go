package pty

type MsgType int

const (
	MsgTypeInit MsgType = iota
	TypeData
	TypeResize
	TypePing
)

type Runner interface {
	ReadPtyAndWriteWs(errorChan chan error)
	ReadWsAndWritePty(errorChan chan error)
	Close()
}
