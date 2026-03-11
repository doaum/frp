package jydiy

import (
	"encoding/binary"
	"errors"
	"io"
)

var (
	ErrMsgType      = errors.New("message type error")
	ErrMaxMsgLength = errors.New("message length exceed the limit")
	ErrMsgLength    = errors.New("message length error")
	ErrMsgFormat    = errors.New("message format error")
)

func (msgCtl *MsgCtl) readMsg(c io.Reader) (typeByte byte, buffer []byte, err error) {
	typeBuf := make([]byte, 1)
	if _, err = io.ReadFull(c, typeBuf); err != nil {
		return
	}
	typeByte = typeBuf[0]
	if _, ok := msgCtl.typeMap[typeByte]; !ok {
		err = ErrMsgType
		return
	}

	var length int64
	if err = binary.Read(c, binary.BigEndian, &length); err != nil {
		return
	}
	if length > msgCtl.maxMsgLength {
		err = ErrMaxMsgLength
		return
	}
	if length < 0 {
		err = ErrMsgLength
		return
	}

	buffer = make([]byte, length)
	n, err := io.ReadFull(c, buffer)
	if err != nil {
		return
	}

	if int64(n) != length {
		err = ErrMsgFormat
	}
	return
}

func (msgCtl *MsgCtl) ReadMsg(c io.Reader) (msg Message, err error) {
	typeByte, buffer, err := msgCtl.readMsg(c)
	if err != nil {
		return
	}
	return msgCtl.UnPack(typeByte, buffer)
}

func (msgCtl *MsgCtl) ReadMsgInto(c io.Reader, msg Message) (err error) {
	_, buffer, err := msgCtl.readMsg(c)
	if err != nil {
		return
	}
	return msgCtl.UnPackInto(buffer, msg)
}

func (msgCtl *MsgCtl) WriteMsg(c io.Writer, msg interface{}) (err error) {
	buffer, err := msgCtl.Pack(msg)
	if err != nil {
		return
	}

	if _, err = c.Write(buffer); err != nil {
		return
	}
	return nil
}
