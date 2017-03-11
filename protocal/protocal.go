package protocal

import (
	"net"
	"time"

	"github.com/juju/errors"
	"github.com/service-exposer/uot/packet"
)

func ServerSide(accept func() (net.Conn, error), target net.Addr) error {
	for {
		conn, err := accept()
		if err != nil {
			return errors.Trace(err)
		}

		pconn, err := net.ListenPacket("udp", "") // listen random address
		if err != nil {
			conn.Close()
			return errors.Trace(err)
		}

		go func() { // conn -> pconn
			defer pconn.Close()
			defer conn.Close()

			p := packet.New()
			for {
				conn.SetReadDeadline(time.Now().Add(Timeout))
				p, err := packet.ReadFrom(conn, p)
				if err != nil {
					return
				}
				conn.SetReadDeadline(time.Time{})

				_, err = pconn.WriteTo(p.Data[:p.Len], target)
				if err != nil {
					return
				}
			}
		}()

		go func() { // pconn <- conn
			defer pconn.Close()
			defer conn.Close()

			p := packet.New()
			for {
				buf := packet.Pool.Get(MaxUDPPacketSize)
				n, _, err := pconn.ReadFrom(buf)
				if err != nil {
					return
				}

				p.Data = buf[:n]
				_, err = packet.WriteTo(conn, p)
				if err != nil {
					return
				}

				packet.Pool.Put(buf)
			}
		}()
	}
}

func ClientSide(nat *NAT, pconn net.PacketConn) error {
	for {
		buf := packet.Pool.Get(MaxUDPPacketSize)
		n, fromAddr, err := pconn.ReadFrom(buf)
		if err != nil {
			return errors.Trace(err)
		}

		data := buf[:n]

		nat.Send(fromAddr, data)
		go func(addr net.Addr) {
			conn := nat.Get(addr)
			if conn == nil {
				return
			}

			defer nat.Remove(addr)
			var (
				err error
				p   = packet.New()
			)
			for {
				conn.SetReadDeadline(time.Now().Add(Timeout))
				p, err = packet.ReadFrom(conn, p)
				if err != nil {
					return
				}
				conn.SetReadDeadline(time.Time{})

				_, err = pconn.WriteTo(p.Data[:p.Len], addr)
				if err != nil {
					return
				}
			}
		}(fromAddr)
	}
}
