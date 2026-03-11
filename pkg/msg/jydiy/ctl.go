package jydiy

import "reflect"

// Message is the common message type accepted by the message codec.
type Message interface{}

const defaultMaxMsgLength = int64(50 * 1024 * 1024)

type MsgCtl struct {
	maxMsgLength int64
	typeMap      map[byte]reflect.Type
	typeByteMap  map[reflect.Type]byte
}

func NewMsgCtl() *MsgCtl {
	return &MsgCtl{
		maxMsgLength: defaultMaxMsgLength,
		typeMap:      make(map[byte]reflect.Type),
		typeByteMap:  make(map[reflect.Type]byte),
	}
}

func (msgCtl *MsgCtl) RegisterMsg(typeByte byte, msg any) {
	t := reflect.TypeOf(msg)
	if t == nil {
		return
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	msgCtl.typeMap[typeByte] = t
	msgCtl.typeByteMap[t] = typeByte
}
