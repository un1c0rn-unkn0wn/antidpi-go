package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
)

var upstreamURL *url.URL

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP address on which the SOCKS5 server listens")
	port := flag.String("port", "8880", "Port on which the SOCKS5 server listens")
	upstream := flag.String("upstream", "", "HTTP proxy to forward traffic to (e.g., http://127.0.0.1:8881)")
	flag.Parse()

	if *upstream == "" {
		log.Fatal("The -upstream flag is required")
	}

	var err error
	upstreamURL, err = url.Parse(*upstream)
	if err != nil {
		log.Fatal("Invalid upstream URL:", err)
	}

	addr := net.JoinHostPort(*ip, *port)
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal("Failed to start server:", err)
	}
	defer listener.Close()

	log.Printf("SOCKS5 server started on %s, upstream: %s", addr, *upstream)

	for {
		conn, err := listener.Accept()
		if err != nil {
			continue
		}
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()

	buf := make([]byte, 256)
	n, err := conn.Read(buf)
	if err != nil {
		return
	}

	if n < 2 || buf[0] != 0x05 {
		return
	}

	nMethods := int(buf[1])
	if n-2 < nMethods {
		return
	}

	methods := buf[2 : 2+nMethods]
	authMethod := byte(0xFF)
	for _, method := range methods {
		if method == 0x00 {
			authMethod = 0x00
			break
		}
	}

	if authMethod == 0xFF {
		return
	}

	_, err = conn.Write([]byte{0x05, authMethod})
	if err != nil {
		return
	}

	n, err = conn.Read(buf)
	if err != nil {
		return
	}

	if n < 7 || buf[0] != 0x05 || buf[1] != 0x01 {
		return
	}

	addrType := buf[3]
	var destAddr string

	switch addrType {
	case 0x01:
		destAddr = net.IP(buf[4:8]).String()
		destAddr = fmt.Sprintf("%s:%d", destAddr, (int(buf[8])<<8)|int(buf[9]))
	case 0x03:
		addrLen := int(buf[4])
		destAddr = string(buf[5 : 5+addrLen])
		destPort := (int(buf[5+addrLen]) << 8) | int(buf[5+addrLen+1])
		destAddr = fmt.Sprintf("%s:%d", destAddr, destPort)
	case 0x04:
		destAddr = net.IP(buf[4:20]).String()
		destAddr = fmt.Sprintf("[%s]:%d", destAddr, (int(buf[20])<<8)|int(buf[21]))
	default:
		return
	}

	_, err = conn.Write([]byte{0x05, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})
	if err != nil {
		return
	}

	proxyConn, err := net.Dial("tcp", upstreamURL.Host)
	if err != nil {
		return
	}
	defer proxyConn.Close()

	connectReq := fmt.Sprintf("CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", destAddr, destAddr)
	_, err = proxyConn.Write([]byte(connectReq))
	if err != nil {
		return
	}

	resp, err := http.ReadResponse(bufio.NewReader(proxyConn), nil)
	if err != nil {
		return
	}
	if resp.StatusCode != 200 {
		return
	}

	go func() {
		io.Copy(proxyConn, conn)
		proxyConn.Close()
	}()
	io.Copy(conn, proxyConn)
}
