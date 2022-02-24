.DEFAULT_GOAL:=local_or_with_proxy

USE_PROXY=GOPROXY=https://goproxy.io
VERSION:=$(shell git describe --abbrev=7 --dirty --always --tags)
BUILD=go build -ldflags "-s -w -X main.Version=$(VERSION)"
BUILD_DIR=build
BIN_NAME=nkn-tunnel
MAIN=bin/main.go
ifdef GOARM
BIN_DIR=$(GOOS)-$(GOARCH)v$(GOARM)
else
BIN_DIR=$(GOOS)-$(GOARCH)
endif

.PHONY: local
local:
	$(BUILD) -o $(BIN_NAME) $(MAIN)

.PHONY: local_with_proxy
local_with_proxy:
	$(USE_PROXY) $(BUILD) -o $(BIN_NAME) $(MAIN)

.PHONY: local_or_with_proxy
local_or_with_proxy:
	${MAKE} local || ${MAKE} local_with_proxy

.PHONY: build
build:
	rm -rf $(BUILD_DIR)/$(BIN_DIR)
	mkdir -p $(BUILD_DIR)/$(BIN_DIR)
	GOOS=$(GOOS) GOARCH=$(GOARCH) $(BUILD) -o $(BUILD_DIR)/$(BIN_DIR)/$(BIN_NAME)$(EXT) $(MAIN)
	${MAKE} zip

.PHONY: tar
tar:
	cd $(BUILD_DIR) && rm -f $(BIN_DIR).tar.gz && tar --exclude ".DS_Store" --exclude "__MACOSX" -czvf $(BIN_DIR).tar.gz $(BIN_DIR)

.PHONY: zip
zip:
	cd $(BUILD_DIR) && rm -f $(BIN_DIR).zip && zip --symlinks --exclude "*.DS_Store*" --exclude "*__MACOSX*" -r $(BIN_DIR).zip $(BIN_DIR)

.PHONY: all
all:
	${MAKE} build GOOS=darwin GOARCH=amd64
	${MAKE} build GOOS=linux GOARCH=amd64
	${MAKE} build GOOS=linux GOARCH=arm64
	${MAKE} build GOOS=linux GOARCH=arm GOARM=5
	${MAKE} build GOOS=linux GOARCH=arm GOARM=6
	${MAKE} build GOOS=linux GOARCH=arm GOARM=7
	${MAKE} build GOOS=windows GOARCH=amd64 EXT=.exe
	${MAKE} build GOOS=windows GOARCH=386 EXT=.exe

.PHONY: ios
ios:
	rm -rf $(BUILD_DIR)/ios Tunnel.xcframework
	mkdir -p $(BUILD_DIR)/ios
	gomobile bind -target=ios -ldflags "-s -w" github.com/nknorg/nkn-tunnel github.com/nknorg/nkn-tuna-session github.com/nknorg/ncp-go github.com/nknorg/tuna github.com/nknorg/nkn-sdk-go github.com/nknorg/nkngomobile
	mv Tunnel.xcframework $(BUILD_DIR)/ios/
	${MAKE} zip BIN_DIR=ios

.PHONY: android
android:
	rm -rf $(BUILD_DIR)/android tunnel.aar tunnel-sources.jar
	mkdir -p $(BUILD_DIR)/android
	gomobile bind -target=android -ldflags "-s -w" github.com/nknorg/nkn-tunnel github.com/nknorg/nkn-tuna-session github.com/nknorg/ncp-go github.com/nknorg/tuna github.com/nknorg/nkn-sdk-go github.com/nknorg/nkngomobile
	mv tunnel.aar tunnel-sources.jar $(BUILD_DIR)/android/
	${MAKE} zip BIN_DIR=android
