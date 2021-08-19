# This Makefile is meant to be used by people that do not usually work
# with Go source code. If you know what GOPATH is then you probably
# don't need to bother with make.

.PHONY: gAVN android ios gAVN-cross evm all test clean
.PHONY: gAVN-linux gAVN-linux-386 gAVN-linux-amd64 gAVN-linux-mips64 gAVN-linux-mips64le
.PHONY: gAVN-linux-arm gAVN-linux-arm-5 gAVN-linux-arm-6 gAVN-linux-arm-7 gAVN-linux-arm64
.PHONY: gAVN-darwin gAVN-darwin-386 gAVN-darwin-amd64
.PHONY: gAVN-windows gAVN-windows-386 gAVN-windows-amd64

GOBIN = ./build/bin
GO ?= latest
GORUN = env GO111MODULE=on go run

gAVN:
	$(GORUN) build/ci.go install ./cmd/gAVN
	@echo "Done building."
	@echo "Run \"$(GOBIN)/gAVN\" to launch gAVN."

all:
	$(GORUN) build/ci.go install

android:
	$(GORUN) build/ci.go aar --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/gAVN.aar\" to use the library."
	@echo "Import \"$(GOBIN)/gAVN-sources.jar\" to add javadocs"
	@echo "For more info see https://stackoverflow.com/questions/20994336/android-studio-how-to-attach-javadoc"

ios:
	$(GORUN) build/ci.go xcode --local
	@echo "Done building."
	@echo "Import \"$(GOBIN)/GAVN.framework\" to use the library."

test: all
	$(GORUN) build/ci.go test

lint: ## Run linters.
	$(GORUN) build/ci.go lint

clean:
	env GO111MODULE=on go clean -cache
	rm -fr build/_workspace/pkg/ $(GOBIN)/*

# The devtools target installs tools required for 'go generate'.
# You need to put $GOBIN (or $GOPATH/bin) in your PATH to use 'go generate'.

devtools:
	env GOBIN= go install golang.org/x/tools/cmd/stringer@latest
	env GOBIN= go install github.com/kevinburke/go-bindata/go-bindata@latest
	env GOBIN= go install github.com/fjl/gencodec@latest
	env GOBIN= go install github.com/golang/protobuf/protoc-gen-go@latest
	env GOBIN= go install ./cmd/abigen
	@type "solc" 2> /dev/null || echo 'Please install solc'
	@type "protoc" 2> /dev/null || echo 'Please install protoc'

# Cross Compilation Targets (xgo)

gAVN-cross: gAVN-linux gAVN-darwin gAVN-windows gAVN-android gAVN-ios
	@echo "Full cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-*

gAVN-linux: gAVN-linux-386 gAVN-linux-amd64 gAVN-linux-arm gAVN-linux-mips64 gAVN-linux-mips64le
	@echo "Linux cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-*

gAVN-linux-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/386 -v ./cmd/gAVN
	@echo "Linux 386 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep 386

gAVN-linux-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/amd64 -v ./cmd/gAVN
	@echo "Linux amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep amd64

gAVN-linux-arm: gAVN-linux-arm-5 gAVN-linux-arm-6 gAVN-linux-arm-7 gAVN-linux-arm64
	@echo "Linux ARM cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep arm

gAVN-linux-arm-5:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-5 -v ./cmd/gAVN
	@echo "Linux ARMv5 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep arm-5

gAVN-linux-arm-6:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-6 -v ./cmd/gAVN
	@echo "Linux ARMv6 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep arm-6

gAVN-linux-arm-7:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm-7 -v ./cmd/gAVN
	@echo "Linux ARMv7 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep arm-7

gAVN-linux-arm64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/arm64 -v ./cmd/gAVN
	@echo "Linux ARM64 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep arm64

gAVN-linux-mips:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips --ldflags '-extldflags "-static"' -v ./cmd/gAVN
	@echo "Linux MIPS cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep mips

gAVN-linux-mipsle:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mipsle --ldflags '-extldflags "-static"' -v ./cmd/gAVN
	@echo "Linux MIPSle cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep mipsle

gAVN-linux-mips64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64 --ldflags '-extldflags "-static"' -v ./cmd/gAVN
	@echo "Linux MIPS64 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep mips64

gAVN-linux-mips64le:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=linux/mips64le --ldflags '-extldflags "-static"' -v ./cmd/gAVN
	@echo "Linux MIPS64le cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-linux-* | grep mips64le

gAVN-darwin: gAVN-darwin-386 gAVN-darwin-amd64
	@echo "Darwin cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-darwin-*

gAVN-darwin-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/386 -v ./cmd/gAVN
	@echo "Darwin 386 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-darwin-* | grep 386

gAVN-darwin-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=darwin/amd64 -v ./cmd/gAVN
	@echo "Darwin amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-darwin-* | grep amd64

gAVN-windows: gAVN-windows-386 gAVN-windows-amd64
	@echo "Windows cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-windows-*

gAVN-windows-386:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/386 -v ./cmd/gAVN
	@echo "Windows 386 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-windows-* | grep 386

gAVN-windows-amd64:
	$(GORUN) build/ci.go xgo -- --go=$(GO) --targets=windows/amd64 -v ./cmd/gAVN
	@echo "Windows amd64 cross compilation done:"
	@ls -ld $(GOBIN)/gAVN-windows-* | grep amd64
