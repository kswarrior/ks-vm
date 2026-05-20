package daemon

import (
	"fmt"
	"io"
	"ksvm/pkg/kvm"
	"net"
	"strings"
	"time"
)

// Mux handles routing internal VM streams.
type Mux struct {
	port    string
	manager *kvm.Manager
}

func NewMux(port string, m *kvm.Manager) *Mux {
	return &Mux{port: port, manager: m}
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

	buf := make([]byte, 1024)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	data := string(buf[:n])
	targetVM := ""
	targetPort := "22"

	lines := strings.Split(data, "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "X-KSVM-Target:") {
			parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(line, "X-KSVM-Target:")), ":")
			if len(parts) >= 1 {
				targetVM = parts[0]
			}
			if len(parts) >= 2 {
				targetPort = parts[1]
			}
		}
	}

	if targetVM == "" {
		io.WriteString(conn, "HTTP/1.1 400 Bad Request\r\n\r\nMissing X-KSVM-Target header")
		return
	}

	info, err := m.manager.Info(targetVM)
	if err != nil || len(info.IPs) == 0 {
		io.WriteString(conn, "HTTP/1.1 404 Not Found\r\n\r\nVM not found or has no IP")
		return
	}

	targetConn, err := net.DialTimeout("tcp", net.JoinHostPort(info.IPs[0], targetPort), 5*time.Second)
	if err != nil {
		io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\n\r\nFailed to connect to guest")
		return
	}
	defer targetConn.Close()

	io.WriteString(conn, "KSVM Mux: Connected to "+targetVM+". Bridging streams...\n")

	done := make(chan bool, 2)
	go func() { io.Copy(targetConn, conn); done <- true }()
	go func() { io.Copy(conn, targetConn); done <- true }()
	<-done
}
