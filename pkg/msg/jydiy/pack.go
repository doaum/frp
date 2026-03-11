package jydiy

import (
	"bytes"
	"encoding/binary"
	"encoding/gob"
	"reflect"
)

func (msgCtl *MsgCtl) unpack(typeByte byte, buffer []byte, msgIn Message) (msg Message, err error) {
	if msgIn == nil {
		t, ok := msgCtl.typeMap[typeByte]
		if !ok {
			err = ErrMsgType
			return
		}

		msg = reflect.New(t).Interface().(Message)
	} else {
		msg = msgIn
	}

	err = gob.NewDecoder(bytes.NewReader(buffer)).Decode(msg)
	return
}

func (msgCtl *MsgCtl) UnPackInto(buffer []byte, msg Message) (err error) {
	_, err = msgCtl.unpack(' ', buffer, msg)
	return
}

func (msgCtl *MsgCtl) UnPack(typeByte byte, buffer []byte) (msg Message, err error) {
	return msgCtl.unpack(typeByte, buffer, nil)
}

func (msgCtl *MsgCtl) Pack(msg Message) ([]byte, error) {
	t := reflect.TypeOf(msg)
	if t == nil {
		return nil, ErrMsgType
	}
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	typeByte, ok := msgCtl.typeByteMap[t]
	if !ok {
		return nil, ErrMsgType
	}

	var contentBuf bytes.Buffer
	if err := gob.NewEncoder(&contentBuf).Encode(msg); err != nil {
		return nil, err
	}
	content := contentBuf.Bytes()

	contentLen := len(content)
	totalLen := 1 + 8 + contentLen
	packet := make([]byte, totalLen)
	packet[0] = typeByte
	binary.BigEndian.PutUint64(packet[1:9], uint64(contentLen))
	copy(packet[9:], content)
	return packet, nil
}
