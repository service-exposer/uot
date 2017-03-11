package protocal

import (
	"encoding/base64"
	"net"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
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

func NewNAT() *NAT {
	return &NAT{
		mu:          new(sync.Mutex),
		ChannelSize: 16,
		Dial:        nil,
		table:       make(map[string]net.Conn),
		sendchans:   make(map[net.Conn]chan<- []byte),
	}
}

func (nat *NAT) Setup(from net.Addr) (ok bool, err error) {
	nat.mu.Lock()
	defer nat.mu.Unlock()
	key := from.Network() + from.String()
	log.WithField("key", key).Debugln("try setup")

	conn, exist := nat.table[key]
	if !exist {
		var err error
		conn, err = nat.Dial()
		if err != nil {
			return false, errors.Trace(err)
		}

		log.WithFields(logrus.Fields{
			"key": key,
		}).Infoln("setup new NAT")

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
		return true, nil
	}
	return false, nil
}
func (nat *NAT) Get(from net.Addr) net.Conn {
	nat.mu.Lock()
	defer nat.mu.Unlock()

	return nat.table[from.Network()+from.String()]
}
func (nat *NAT) Send(from net.Addr, data []byte) error {
	nat.mu.Lock()
	defer nat.mu.Unlock()

	conn := nat.table[from.Network()+from.String()]
	sendch := nat.sendchans[conn]
	select {
	case sendch <- data:
		log.WithField("tcp", conn.RemoteAddr()).
			WithField("data", base64.StdEncoding.EncodeToString(data)).
			Info("write to TCP")
		return nil
	default:
		err := errors.New("send channel buffer filled")
		log.WithField("tcp", conn.RemoteAddr()).
			WithField("data", base64.StdEncoding.EncodeToString(data)).
			WithField("err", errors.ErrorStack(errors.Trace(err))).
			Info("write error")

		return errors.Trace(err)
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

	log.WithFields(logrus.Fields{
		"key": key,
	}).Infoln("remove NAT")
}
