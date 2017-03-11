package protocal

import (
	"net"
	"sync"
	"time"

	"github.com/juju/errors"
	"github.com/service-exposer/uot/packet"
)

const MaxUDPPacketSize = 65535
const Timeout = time.Second * 30

type NAT struct {
	mu *sync.Mutex // protect all fields

	ChannelSize int
	Dial        func() (net.Conn, error) // Just modify before use it

	table     map[string]net.Conn // net.Addr.Network() + net.Addr.String()
	sendchans map[net.Conn]chan<- []byte
}

func (nat *NAT) Setup(from net.Addr) error {
	nat.mu.Lock()
	defer nat.mu.Unlock()

	key := from.Network() + from.String()

	conn, exist := nat.table[key]
	if !exist {
		var err error
		conn, err = nat.Dial()
		if err != nil {
			return errors.Trace(err)
		}

		ch := make(chan []byte, nat.ChannelSize)
		nat.table[key] = conn
		nat.sendchans[conn] = ch

		go func(nat *NAT, key string, conn net.Conn, recv <-chan []byte) { // send loop
			defer nat.Remove(from)

			isDone := false
			p := packet.New()
			for data := range recv {
				if isDone {
					continue
				}
				p.Reset()
				p.Data = data

				conn.SetWriteDeadline(time.Now().Add(Timeout))
				_, err := packet.WriteTo(conn, p)
				if err != nil {
					isDone = true
					continue
				}
				conn.SetWriteDeadline(time.Time{})
			}
		}(nat, key, conn, ch)
	}
	return nil
}
func (nat *NAT) Get(from net.Addr) net.Conn {
	nat.mu.Lock()
	defer nat.mu.Unlock()

	return nat.table[from.Network()+from.String()]
}
func (nat *NAT) Send(from net.Addr, data []byte) error {
	nat.mu.Lock()
	defer nat.mu.Unlock()

	err := nat.Setup(from)
	if err != nil {
		return errors.Trace(err)
	}

	conn := nat.table[from.Network()+from.String()]
	sendch := nat.sendchans[conn]
	select {
	case sendch <- data:
		return nil
	default:
		return errors.New("send channel buffer filled")
	}
}

func (nat *NAT) Remove(addr net.Addr) {
	nat.mu.Lock()
	defer nat.mu.Unlock()

	key := addr.Network() + addr.String()

	conn, exist := nat.table[key]
	if !exist {
		return
	}
	delete(nat.table, key)
	delete(nat.sendchans, conn)
}
