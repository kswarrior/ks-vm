package daemon

import (
	"fmt"
	"io"
	"net"
	"strings"
)

// Mux handles routing internal VM streams.
type Mux struct {
	port string
}

func NewMux(port string) *Mux {
	return &Mux{port: port}
}

func (m *Mux) Start() error {
	l, err := net.Listen("tcp", ":"+m.port)
	if err != nil {
		return err
	}
	fmt.Printf("Gateway Multiplexer listening on port %s\n", m.port)

	for {
		conn, err := l.Accept()
		if err != nil {
			fmt.Printf("Mux accept error: %v\n", err)
			continue
		}
		go m.handleConn(conn)
	}
}

func (m *Mux) handleConn(conn net.Conn) {
	defer conn.Close()

	// Basic routing/multiplexing based on initial data
	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	header := string(buf[:n])
	// Example: Bridge to internal target VM based on a header or path
	if strings.Contains(header, "X-KSVM-Target:") {
		// Implement bridging logic here
		fmt.Println("Bridging connection based on header...")
	}

	// Placeholder for stream proxying
	io.WriteString(conn, "KSVM Gateway Multiplexer v0.1\n")
}
