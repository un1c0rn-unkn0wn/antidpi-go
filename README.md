
# antidpi-go

A lightweight HTTP TCP proxy designed to fragment TLS traffic and help bypass Deep Packet Inspection (DPI) systems.

🔧 Built in **Go**, this proxy supports selective TLS fragmentation and outgoing IP binding. It can be used to bypass censorship and test services for compliance with various standards.

 ⚠️**WARNING!** 
 All code is written using ChatGPT and DeepSeek.
 
**Note:** This project is intended for educational and research purposes only.

## Features

- ✅ Fragmentation of TLS traffic for selected ports (e.g. 443, 8443, etc.)
- ✅ Configurable listening IP and port
- ✅ Configurable outgoing (bind) IP
- ✅ Lightweight and dependency-free
- ✅ Randomized TLS fragment sizes

## How it works

The proxy listens for incoming HTTP `CONNECT` requests and forwards them to the target destination. If the port matches the list of fragmentation ports (e.g., 443), it intercepts and fragments the initial TLS handshake, making detection by DPI systems more difficult.

## Build Instructions

### From Source

```bash
go build -o antidpi-go main.go
```

### Using Makefile

```bash
make        # Builds binaries for Windows/Linux/macOS/FreeBSD/Darwin
make clean  # Removes binaries and build artifacts
make zip	# Archiving binaries for different architectures into ZIP archive
```

## Usage

```bash
./antidpi-go -ip IP_FOR_LISTENING -port PORT_FOR_LISTENING -outgoing-addr YOUR_INTERFACE_IP -fragment-ports="443,8443"
```

### Flags

| Flag               | Description                                            | Default         |
|--------------------|--------------------------------------------------------|-----------------|
| `-ip`              | IP address to listen on                                | `127.0.0.1`     |
| `-port`            | TCP port to listen on                                  | `8881`          |
| `-outgoing-addr`   | Local IP to bind for outgoing connections              | `-`
| `-fragment-ports`  | Comma-separated list of destination ports to fragment  | `443`

### Example

```bash
./antidpi-go -ip 127.0.0.1 -port 8881 -fragment-ports="443,8443"
```

This starts a proxy on `127.0.0.1:8881` and fragments TLS traffic to ports `443` and `8443`.

###  Graceful Shutdown

Press `Ctrl+C` or send `SIGINT/SIGTERM` to stop the proxy. It will wait for all active connections to close before exiting.

##  Inspirations
 [GVCoder09/NoDPI](https://github.com/GVCoder09/NoDPI) - Author of the original idea and logic

## License

MIT License
