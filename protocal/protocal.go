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

func ServerSide(accept func() (net.Conn, error), target net.Addr) error {
	for {
		conn, err := accept()
		if err != nil {
			return errors.Trace(err)
		}
		log.WithField("remote", conn.RemoteAddr()).
			Info("new conn")

		pconn, err := net.ListenPacket("udp", "localhost:") // listen random address
		if err != nil {
			conn.Close()
			return errors.Trace(err)
		}
		log.WithField("addr", pconn.LocalAddr()).
			Info("listen random UDP addr")

		closeOnce := new(sync.Once)
		closeConns := func() {
			closeOnce.Do(func() {
				pconn.Close()
				conn.Close()
				log.WithFields(logrus.Fields{
					"conn":  conn.RemoteAddr(),
					"pconn": conn.LocalAddr(),
				}).Infoln("close pair")
			})
		}

		go func(conn net.Conn, pconn net.PacketConn, shutdown func()) { // conn -> pconn
			defer shutdown()

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

				log.WithFields(logrus.Fields{
					"from": conn.RemoteAddr(),
					"via":  pconn.LocalAddr(),
					"data": base64.StdEncoding.EncodeToString(p.Data[:p.Len]),
				}).Infoln("forward tcp -> udp")
			}
		}(conn, pconn, closeConns)

		go func(conn net.Conn, pconn net.PacketConn, shutdown func()) { // pconn <- conn
			defer shutdown()

			buf := make([]byte, MaxUDPPacketSize)
			for {
				pconn.SetReadDeadline(time.Now().Add(Timeout))
				n, _, err := pconn.ReadFrom(buf)
				if err != nil {
					log.Error(errors.ErrorStack(errors.Trace(err)))
					return
				}
				pconn.SetReadDeadline(time.Time{})

				data := buf[:n]

				conn.SetWriteDeadline(time.Now().Add(Timeout))
				err = packet.Write(conn, data)
				if err != nil {
					log.Error(errors.ErrorStack(errors.Trace(err)))
					return
				}
				conn.SetWriteDeadline(time.Time{})

				log.WithFields(logrus.Fields{
					"from": pconn.LocalAddr(),
					"to":   conn.RemoteAddr(),
					"data": base64.StdEncoding.EncodeToString(data),
				}).Infoln("forward udp -> tcp")
				packet.Pool.Put(buf)
			}
		}(conn, pconn, closeConns)
	}
}

func ClientSide(nat *NAT, pconn net.PacketConn) error {
	for {
		buf := make([]byte, MaxUDPPacketSize)
		n, fromAddr, err := pconn.ReadFrom(buf)
		if err != nil {
			return errors.Trace(err)
		}

		data := buf[:n]

		isNewFromAddr, err := nat.Setup(fromAddr)
		if err != nil {
			return errors.Trace(err)
		}
		err = nat.Send(fromAddr, data)
		if err != nil {
			return errors.Trace(err)
		}

		if !isNewFromAddr {
			continue
		}

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
					log.WithFields(logrus.Fields{
						"from":  conn.LocalAddr(),
						"err":   err,
						"stack": errors.ErrorStack(errors.Trace(err)),
					}).Warnln("read tcp")

					return
				}
				conn.SetReadDeadline(time.Time{})

				_, err = pconn.WriteTo(p.Data[:p.Len], addr)
				if err != nil {
					log.WithFields(logrus.Fields{
						"to":    addr,
						"err":   err,
						"stack": errors.ErrorStack(errors.Trace(err)),
					}).Warnln("write udp")
					return
				}

				log.WithFields(logrus.Fields{
					"from": conn.LocalAddr(),
					"to":   addr.String(),
					"data": base64.StdEncoding.EncodeToString(p.Data[:p.Len]),
				}).Infoln("forward tcp -> udp")

			}
		}(fromAddr)
	}
}
