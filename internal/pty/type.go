package pty

type MsgType int

const (
	MsgTypeInit MsgType = iota
	TypeData
	TypeResize
	TypePing
)
