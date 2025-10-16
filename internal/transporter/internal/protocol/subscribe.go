package protocol

import (
	"encoding/binary"
	"io"

	"github.com/devagame/due/v2/core/buffer"
	"github.com/devagame/due/v2/errors"
	"github.com/devagame/due/v2/internal/transporter/internal/route"
	"github.com/devagame/due/v2/session"
)

const (
	subscribeReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + b8 + b16
	subscribeResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

// EncodeSubscribeReq 编码订阅频道请求（单次最多订阅65535个对象）
// 协议：size + header + route + seq + session kind + count + targets + channel
func EncodeSubscribeReq(seq uint64, kind session.Kind, targets []int64, channel string) *buffer.NocopyBuffer {
	size := subscribeReqBytes + len(targets)*8 + len([]byte(channel))

	writer := buffer.MallocWriter(size)
	writer.WriteUint32s(binary.BigEndian, uint32(size-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Subscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint8s(uint8(kind))
	writer.WriteUint16s(binary.BigEndian, uint16(len(targets)))
	writer.WriteInt64s(binary.BigEndian, targets...)
	writer.WriteString(channel)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeSubscribeReq 解码订阅频道请求
// 协议：size + header + route + seq + session kind + count + targets + channel
func DecodeSubscribeReq(data []byte) (seq uint64, kind session.Kind, targets []int64, channel string, err error) {
	reader := buffer.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes, io.SeekStart); err != nil {
		return
	}

	if seq, err = reader.ReadUint64(binary.BigEndian); err != nil {
		return
	}

	var k uint8
	if k, err = reader.ReadUint8(); err != nil {
		return
	} else {
		kind = session.Kind(k)
	}

	count, err := reader.ReadUint16(binary.BigEndian)
	if err != nil {
		return
	}

	if targets, err = reader.ReadInt64s(binary.BigEndian, int(count)); err != nil {
		return
	}

	channel = string(data[subscribeReqBytes+8*count:])

	return
}

// EncodeSubscribeRes 编码订阅频道响应
// 协议：size + header + route + seq + code
func EncodeSubscribeRes(seq uint64, code uint16) *buffer.NocopyBuffer {
	writer := buffer.MallocWriter(subscribeResBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(subscribeResBytes-defaultSizeBytes))
	writer.WriteUint8s(dataBit)
	writer.WriteUint8s(route.Subscribe)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteUint16s(binary.BigEndian, code)

	return buffer.NewNocopyBuffer(writer)
}

// DecodeSubscribeRes 解码订阅频道响应
// 协议：size + header + route + seq + code
func DecodeSubscribeRes(data []byte) (code uint16, err error) {
	if len(data) != subscribeResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := buffer.NewReader(data)

	if _, err = reader.Seek(-defaultCodeBytes, io.SeekEnd); err != nil {
		return
	}

	if code, err = reader.ReadUint16(binary.BigEndian); err != nil {
		return
	}

	return
}
