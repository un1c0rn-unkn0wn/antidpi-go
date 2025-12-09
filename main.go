package main

import (
    "bufio"
    "bytes"
    "context"
    "flag"
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

const (
    ConnTimeout     = 90 * time.Second
    MaxRequestSize  = 1024 * 1024
)

var (
    maxConnections = make(chan struct{}, 2000)
    dnsHosts map[string]string
)

var wg sync.WaitGroup

func init() {
    rand.Seed(time.Now().UnixNano())
}

func loadHostsFile(path string) map[string]string {
    result := make(map[string]string)
    file, err := os.Open(path)
    if err != nil {
        log.Printf("Failed to open dns-hosts file: %v", err)
        return result
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    for scanner.Scan() {
        line := strings.TrimSpace(scanner.Text())
        if line == "" || strings.HasPrefix(line, "#") {
            continue
        }
        fields := strings.Fields(line)
        if len(fields) >= 2 {
            ip := fields[0]
            for _, host := range fields[1:] {
                result[strings.ToLower(host)] = ip
            }
        }
    }
    if err := scanner.Err(); err != nil {
        log.Printf("Failed reading dns-hosts file: %v", err)
    }
    return result
}

func main() {
    ip := flag.String("ip", "127.0.0.1", "IP address to listen on")
    port := flag.String("port", "8881", "Port to listen on")
    outgoingAddr := flag.String("outgoing-addr", "", "IP for outgoing connections")
    fragmentPortsStr := flag.String("fragment-ports", "443", "Ports to fragment")
    dnsHostsPath := flag.String("dns-hosts", "", "Path to custom DNS hosts file")
    flag.Parse()

    if *dnsHostsPath != "" {
        dnsHosts = loadHostsFile(*dnsHostsPath)
        log.Printf("Loaded %d host overrides from %s", len(dnsHosts), *dnsHostsPath)
    } else {
        dnsHosts = make(map[string]string)
    }

    fragmentPorts := parsePortList(*fragmentPortsStr)
    listenAddr := net.JoinHostPort(*ip, *port)
    listener, err := net.Listen("tcp", listenAddr)
    if err != nil {
        log.Fatalf("Failed to start server: %v", err)
    }
    defer listener.Close()

    log.Println("HTTP proxy is running on", listenAddr)
    log.Println("---------------------------------------")
    log.Println("Connections timeout:", ConnTimeout)
    log.Println("Max connections:", cap(maxConnections))
    log.Println("Max request size:", MaxRequestSize/1024, "KB")
    log.Println("---------------------------------------")

    ctx, cancel := context.WithCancel(context.Background())
    go handleSignals(cancel, listener)

    for {
        conn, err := listener.Accept()
        if err != nil {
            select {
            case <-ctx.Done():
                log.Println("Shutdown signal received. Waiting...")
                wg.Wait()
                return
            default:
                log.Printf("Failed to accept connection: %v", err)
                continue
            }
        }

        select {
        case maxConnections <- struct{}{}:
        default:
            log.Println("Too many connections, rejecting")
            conn.Close()
            continue
        }

        wg.Add(1)
        go func(c net.Conn) {
            defer wg.Done()
            defer func() { <-maxConnections }()
            handleConnection(c, *outgoingAddr, fragmentPorts)
        }(conn)
    }
}

func resolveWithHosts(original string) string {
    host, port, err := net.SplitHostPort(original)
    if err != nil {
        host = original
        port = "80"
    }
    low := strings.ToLower(host)
    if ip, ok := dnsHosts[low]; ok {
        return net.JoinHostPort(ip, port)
    }
    return original
}

func handleConnection(localConn net.Conn, outgoingIPStr string, fragmentPorts map[string]bool) {
    defer localConn.Close()
    localConn.SetDeadline(time.Now().Add(ConnTimeout))

    buf := make([]byte, 1500)
    n, err := localConn.Read(buf)
    if err != nil || n == 0 {
        return
    }

    if n > MaxRequestSize {
        log.Printf("Request too large: %d bytes", n)
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

    // CONNECT logic
    if method == "CONNECT" {
        hostPort := string(parts[1])
        hostPort = resolveWithHosts(hostPort)

        _, port, err := net.SplitHostPort(hostPort)
        if err != nil {
            hostPort = net.JoinHostPort(hostPort, "443")
            port = "443"
        }

        _, err = localConn.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
        if err != nil {
            return
        }

        dialer := &net.Dialer{Timeout: 10 * time.Second}
        if outgoingIPStr != "" {
            dialer = &net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(outgoingIPStr)}, Timeout: 10 * time.Second}
        }

        remoteConn, err := dialer.Dial("tcp", hostPort)
        if err != nil {
            log.Printf("Failed to connect to %s: %v", hostPort, err)
            return
        }
        defer remoteConn.Close()
        remoteConn.SetDeadline(time.Now().Add(ConnTimeout))

        if fragmentPorts[port] {
            if !forwardWithFragmentation(localConn, remoteConn) {
                return
            }
        }

        go io.Copy(remoteConn, localConn)
        io.Copy(localConn, remoteConn)
        return
    }

    // HTTP request
    hostPort := ""
    headers := bytes.Split(buf[:n], []byte("\r\n"))
    for _, header := range headers[1:] {
        if len(header) > 5 && bytes.Equal(bytes.ToLower(header[:5]), []byte("host:")) {
            hostPort = string(bytes.TrimSpace(header[5:]))
            break
        }
    }

    if hostPort == "" && len(parts) >= 2 {
        urlPart := string(parts[1])
        if strings.HasPrefix(urlPart, "http://") {
            u, err := url.Parse(urlPart)
            if err == nil {
                hostPort = u.Host
            }
        }
    }

    if hostPort == "" {
        log.Println("Host not found in request")
        return
    }

    if _, _, err := net.SplitHostPort(hostPort); err != nil {
        hostPort = net.JoinHostPort(hostPort, "80")
    }

    hostPort = resolveWithHosts(hostPort)

    dialer := &net.Dialer{Timeout: 10 * time.Second}
    if outgoingIPStr != "" {
        dialer = &net.Dialer{LocalAddr: &net.TCPAddr{IP: net.ParseIP(outgoingIPStr)}, Timeout: 10 * time.Second}
    }

    remoteConn, err := dialer.Dial("tcp", hostPort)
    if err != nil {
        log.Printf("Failed to connect to %s: %v", hostPort, err)
        return
    }
    defer remoteConn.Close()
    remoteConn.SetDeadline(time.Now().Add(ConnTimeout))

    _, err = remoteConn.Write(buf[:n])
    if err != nil {
        log.Printf("Failed to send request: %v", err)
        return
    }

    go io.Copy(remoteConn, localConn)
    io.Copy(localConn, remoteConn)
}

func forwardWithFragmentation(src net.Conn, dst net.Conn) bool {
    head := make([]byte, 5)
    _, err := io.ReadFull(src, head)
    if err != nil { return false }

    data := make([]byte, 2048)
    n, err := src.Read(data)
    if err != nil && err != io.EOF { return false }
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
        if err != nil { return false }
    }

    return true
}

func randInt(min int, max int) int {
    if max <= min { return min }
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

func parsePortList(s string) map[string]bool {
    result := make(map[string]bool)
    for _, part := range strings.Split(s, ",") {
        p := strings.TrimSpace(part)
        if p != "" { result[p] = true }
    }
    return result
}

func handleSignals(cancel context.CancelFunc, listener net.Listener) {
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    sig := <-sigChan
    log.Printf("Received termination signal: %s", sig)
    cancel()
    listener.Close()
}

