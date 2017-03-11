package packet

import (
	"encoding/binary"
	"io"

	"github.com/juju/errors"
)

var Pool BytesPool

func init() {
	Pool = NewBytesPool(1024)
}

type BytesPool interface {
	Get(len int) []byte
	Put(p []byte)
}

type bytesPool struct {
	ch chan []byte
}

func (p *bytesPool) Get(min int) []byte {
	for {
		select {
		case buf := <-p.ch:
			if cap(buf) >= min {
				return buf[:cap(buf)]
			}
			if len(p.ch) < cap(p.ch)/2 {
				p.Put(buf)
			}
		default:
			return make([]byte, min)
		}
	}
}

func (p *bytesPool) Put(buf []byte) {
	select {
	case p.ch <- buf:
	default:
	}
}

func NewBytesPool(size int) BytesPool {
	const (
		MinSize = 16
	)

	if size < MinSize {
		size = MinSize
	}
	return &bytesPool{
		ch: make(chan []byte, size),
	}
}

type Packet struct {
	ID   uint32
	Len  int
	Data []byte
}

func New() *Packet {
	return &Packet{
		ID:   0,
		Len:  0,
		Data: nil,
	}
}

// ReadFrom read packet from r
// BigEndian(uint16(len)) + BigEndian(uint64(id)) + data
// len = 2 + 4 + len(data)
func ReadFrom(r io.Reader, reuse *Packet) (p *Packet, err error) {
	if reuse != nil {
		reuse.Reset()
		p = reuse
	} else {
		p = New()
	}

	var (
		len uint16
	)
	// read len
	err = binary.Read(r, binary.BigEndian, &len)
	if err != nil {
		return nil, errors.Trace(err)
	}

	if len < 2+4 {
		panic("bad protocal")
	}
	p.Len = int(len - (2 + 4))

	// read id
	err = binary.Read(r, binary.BigEndian, &p.ID)
	if err != nil {
		return nil, errors.Trace(err)
	}

	// read data
	if p.Len == 0 {
		return p, nil
	}

	p.Data = Pool.Get(p.Len)[:p.Len]
	_, err = io.ReadAtLeast(r, p.Data, p.Len)
	return p, errors.Trace(err)
}

// WriteTo write package to w
// Please goto method ReadFrom to see protocal details
func WriteTo(w io.Writer, p *Packet) (int64, error) {
	if 2+4+len(p.Data) > 65535 {
		return 0, errors.New("too big data size")
	}
	p.Len = int(2 + 4 + len(p.Data))

	err := binary.Write(w, binary.BigEndian, uint16(p.Len))
	if err != nil {
		return 0, errors.Trace(err)
	}
	err = binary.Write(w, binary.BigEndian, p.ID)
	if err != nil {
		return 0, errors.Trace(err)
	}
	n, err := w.Write(p.Data)
	return int64(n), errors.Trace(err)
}

// Reset reuse Packet
func (p *Packet) Reset() {
	p.ID = 0
	p.Len = 0
	if p.Data != nil {
		Pool.Put(p.Data)
		p.Data = nil
	}
}