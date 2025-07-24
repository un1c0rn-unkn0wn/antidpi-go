package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

var wg sync.WaitGroup

func init() {
	rand.Seed(time.Now().UnixNano())
}

func main() {
	ip := flag.String("ip", "127.0.0.1", "IP address to listen on")
	port := flag.String("port", "8881", "Port to listen on")
	outgoingAddr := flag.String("outgoing-addr", "", "IP address for outgoing connections")
	fragmentPortsStr := flag.String("fragment-ports", "443", "Ports to fragment traffic on, comma separated (e.g., 443,8443)")
	flag.Parse()

	fragmentPorts := parsePortList(*fragmentPortsStr)

	listenAddr := net.JoinHostPort(*ip, *port)
	listener, err := net.Listen("tcp", listenAddr)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
	defer listener.Close()

	fmt.Printf("Proxy is running on %s\n", listenAddr)

	ctx, cancel := context.WithCancel(context.Background())
	go handleSignals(cancel, listener)

	for {
		conn, err := listener.Accept()
		if err != nil {
			select {
			case <-ctx.Done():
				log.Println("Shutdown signal received. Waiting for connections to finish...")
				wg.Wait()
				log.Println("All connections closed. Exiting.")
				return
			default:
				log.Printf("Failed to accept connection: %v", err)
				continue
			}
		}
		wg.Add(1)
		go func(c net.Conn) {
			defer wg.Done()
			handleConnection(c, *outgoingAddr, fragmentPorts)
		}(conn)
	}
}

func handleSignals(cancel context.CancelFunc, listener net.Listener) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigChan
	log.Printf("Received termination signal: %s", sig)
	cancel()
	listener.Close()
}

func parsePortList(s string) map[string]bool {
	result := make(map[string]bool)
	for _, part := range strings.Split(s, ",") {
		p := strings.TrimSpace(part)
		if p != "" {
			result[p] = true
		}
	}
	return result
}

func handleConnection(localConn net.Conn, outgoingIPStr string, fragmentPorts map[string]bool) {
    defer localConn.Close()

    buf := make([]byte, 1500)
    n, err := localConn.Read(buf)
    if err != nil || n == 0 {
        return
    }

    line := buf[:n]
    firstLineEnd := findLineEnd(line)
    if firstLineEnd == -1 {
        return
    }

    parts := splitBySpace(line[:firstLineEnd])
    if len(parts) < 2 {
        return
    }

    method := string(parts[0])

    // Обработка CONNECT (HTTPS)
    if method == "CONNECT" {
        hostPort := string(parts[1])
        remoteAddr := hostPort
        _, port, err := net.SplitHostPort(remoteAddr)
        if err != nil {
            remoteAddr = net.JoinHostPort(remoteAddr, "443")
            port = "443"
        }

        _, err = localConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
        if err != nil {
            return
        }

        dialer := &net.Dialer{Timeout: 10 * time.Second}
        if outgoingIPStr != "" {
            localAddr := &net.TCPAddr{IP: net.ParseIP(outgoingIPStr)}
            dialer = &net.Dialer{LocalAddr: localAddr, Timeout: 10 * time.Second}
        }

        remoteConn, err := dialer.Dial("tcp", remoteAddr)
        if err != nil {
            log.Printf("Failed to connect to %s: %v", remoteAddr, err)
            return
        }
        defer remoteConn.Close()

        if fragmentPorts[port] {
            ok := forwardWithFragmentation(localConn, remoteConn)
            if !ok {
                return
            }
        }

        go io.Copy(remoteConn, localConn)
        io.Copy(localConn, remoteConn)
        return
    }

    // Обработка обычного HTTP-запроса
    hostPort := ""
    
    // Ищем Host в заголовках
    headers := bytes.Split(buf[:n], []byte("\r\n"))
    for _, header := range headers[1:] { // Пропускаем первую строку
        if len(header) > 5 && bytes.Equal(bytes.ToLower(header[:5]), []byte("host:")) {
            hostPort = string(bytes.TrimSpace(header[5:]))
            break
        }
    }
    
    // Если Host не найден в заголовках, пробуем из URL в первой строке
    if hostPort == "" {
        if len(parts) >= 2 {
            urlPart := string(parts[1])
            if strings.HasPrefix(urlPart, "http://") {
                u, err := url.Parse(urlPart)
                if err == nil {
                    hostPort = u.Host
                }
            }
        }
    }

    if hostPort == "" {
        log.Println("Host not found in HTTP request")
        return
    }

    // Если порт не указан, добавляем 80
    if _, _, err := net.SplitHostPort(hostPort); err != nil {
        hostPort = net.JoinHostPort(hostPort, "80")
    }

    dialer := &net.Dialer{Timeout: 10 * time.Second}
    if outgoingIPStr != "" {
        localAddr := &net.TCPAddr{IP: net.ParseIP(outgoingIPStr)}
        dialer = &net.Dialer{LocalAddr: localAddr, Timeout: 10 * time.Second}
    }

    remoteConn, err := dialer.Dial("tcp", hostPort)
    if err != nil {
        log.Printf("Failed to connect to %s: %v", hostPort, err)
        return
    }
    defer remoteConn.Close()

    // Отправляем оригинальный запрос
    _, err = remoteConn.Write(buf[:n])
    if err != nil {
        log.Printf("Failed to send request to remote: %v", err)
        return
    }

    // Проксируем трафик в обе стороны
    go io.Copy(remoteConn, localConn)
    io.Copy(localConn, remoteConn)
}

func forwardWithFragmentation(src net.Conn, dst net.Conn) bool {
	head := make([]byte, 5)
	_, err := io.ReadFull(src, head)
	if err != nil {
		return false
	}

	data := make([]byte, 2048)
	n, err := src.Read(data)
	if err != nil && err != io.EOF {
		return false
	}
	data = data[:n]

	payload := append([]byte{}, data...)

	for len(payload) > 0 {
		partLen := randInt(1, len(payload))
		part := payload[:partLen]
		payload = payload[partLen:]

		header := []byte{0x16, 0x03, 0x04}
		length := []byte{byte(len(part) >> 8), byte(len(part) & 0xff)}
		fragment := append(append(header, length...), part...)

		_, err := dst.Write(fragment)
		if err != nil {
			return false
		}
	}

	return true
}

func randInt(min int, max int) int {
	if max <= min {
		return min
	}
	return rand.Intn(max-min+1) + min
}

func findLineEnd(data []byte) int {
	for i := 0; i < len(data)-1; i++ {
		if data[i] == '\r' && data[i+1] == '\n' {
			return i
		}
	}
	return -1
}

func splitBySpace(line []byte) [][]byte {
	var res [][]byte
	start := 0
	for i, b := range line {
		if b == ' ' {
			res = append(res, line[start:i])
			start = i + 1
		}
	}
	res = append(res, line[start:])
	return res
}
