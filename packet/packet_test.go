package packet

import (
	"log"
	"net"
	"sync"
	"testing"

	"github.com/juju/errors"
)

func TestPacket(t *testing.T) {
	s, c := net.Pipe()
	const N int = 1000

	wg := new(sync.WaitGroup)
	wg.Add(2)
	go func(conn net.Conn) {
		defer wg.Done()

		p := New()
		for i := 0; i < N; i++ {
			p.Reset()
			p, err := ReadFrom(conn, p)
			if err != nil {
				log.Fatal(errors.ErrorStack(errors.Trace(err)))
			}

			if p.Len != 5 {
				log.Fatal("expect 5 got ", p.Len)
			}
			if string(p.Data) != "hello" {
				log.Fatal("expect hello got ", string(p.Data))
			}
		}
	}(s)

	go func(conn net.Conn) {
		defer wg.Done()

		p := New()
		for i := 0; i < N; i++ {
			p.Reset()
			p.Data = []byte("hello")

			_, err := WriteTo(c, p)
			if err != nil {
				log.Fatal(errors.ErrorStack(errors.Trace(err)))
			}
		}
	}(c)

	wg.Wait()
}
