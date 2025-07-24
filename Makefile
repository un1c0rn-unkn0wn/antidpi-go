VERSION=v1.1

default:
	@echo "=============Building binaries============="

	# Linux 386
	CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_386 main.go

	# Linux amd64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_amd64 main.go

	# Linux arm
	CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_arm main.go

	# Linux arm64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_arm64 main.go

	# Darwin amd64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.darwin_amd64 main.go

	# Darwin arm64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.darwin_arm64 main.go

	# FreeBSD 386
	CGO_ENABLED=0 GOOS=freebsd GOARCH=386 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.freebsd_386 main.go

	# FreeBSD amd64
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.freebsd_amd64 main.go

	# FreeBSD arm
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.freebsd_arm main.go

	# FreeBSD arm64
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.freebsd_arm64 main.go
	
	# !EXPEREMENTAL!
	# Linux mips-softfloat
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=softfloat go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_mips_softfloat main.go
	
	# Linux mips-hardfloat
	CGO_ENABLED=0 GOOS=linux GOARCH=mips GOMIPS=hardfloat go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_mips_hardfloat main.go
	
	# Linux mipsle-softfloat
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=softfloat go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_mipsle_softfloat main.go
	
	# Linux mipsle-hardfloat
	CGO_ENABLED=0 GOOS=linux GOARCH=mipsle GOMIPS=hardfloat go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_mipsle_hardfloat main.go
	
	# Linux mips64
	CGO_ENABLED=0 GOOS=linux GOARCH=mips64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_mips64 main.go
	
	# Linux mips64le
	CGO_ENABLED=0 GOOS=linux GOARCH=mips64le go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.linux_mips64le main.go
	
	# Windows amd64
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.windows_amd64.exe main.go
	
	# Windows arm64
	CGO_ENABLED=0 GOOS=windows GOARCH=arm64 go build -ldflags="-s -w -X 'main.Version=$(VERSION)'" -o build/antidpi-go.windows_arm64.exe main.go

	@echo "==============Adding License==============="
	# Adding License
	cp LICENSE build/LICENSE

	@echo "=============Building complete============="
	
clean:
	@echo "==========Cleaning build directory========="
	rm -f build/*
	rm -r build
	@echo "=============Cleaning complete============="
	
zip:
	@echo "=========Archiving build directory========="
	zip -r antidpi-go.zip build/*
	@echo "============Archiving complete============="
