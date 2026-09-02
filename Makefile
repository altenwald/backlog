.PHONY: build test bundle clean

APP_NAME = Backlog
BUNDLE_DIR = bin/$(APP_NAME).app
MIN_MACOS_VER = 11.0
MACOS_ENV = CGO_ENABLED=1 GOOS=darwin MACOSX_DEPLOYMENT_TARGET=$(MIN_MACOS_VER) CGO_CFLAGS="-mmacosx-version-min=$(MIN_MACOS_VER)" CGO_LDFLAGS="-mmacosx-version-min=$(MIN_MACOS_VER)"

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
GIT_COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
VERSION_NUM = $(shell echo $(VERSION) | sed 's/^v//' | cut -d'-' -f1)

LDFLAGS = -s -w \
	-X github.com/altenwald/backlog/pkg/version.Version=$(VERSION) \
	-X github.com/altenwald/backlog/pkg/version.GitCommit=$(GIT_COMMIT) \
	-X github.com/altenwald/backlog/pkg/version.BuildDate=$(BUILD_DATE)

build:
	$(MACOS_ENV) go build -ldflags="$(LDFLAGS)" -o bin/backlog ./cmd/backlog

test:
	go test -v ./...

bundle:
	@echo "Building Universal binary for macOS $(MIN_MACOS_VER)+ (arm64 + amd64) [Version: $(VERSION)]..."
	@mkdir -p bin
	@$(MACOS_ENV) GOARCH=arm64 go build -ldflags="$(LDFLAGS)" -o bin/backlog_arm64 ./cmd/backlog
	@$(MACOS_ENV) GOARCH=amd64 go build -ldflags="$(LDFLAGS)" -o bin/backlog_amd64 ./cmd/backlog
	@lipo -create -output bin/backlog bin/backlog_amd64 bin/backlog_arm64
	@rm -f bin/backlog_arm64 bin/backlog_amd64
	@echo "Bundling $(BUNDLE_DIR)..."
	@rm -rf $(BUNDLE_DIR)
	@mkdir -p $(BUNDLE_DIR)/Contents/MacOS
	@mkdir -p $(BUNDLE_DIR)/Contents/Resources
	@cp bin/backlog $(BUNDLE_DIR)/Contents/MacOS/backlog
	@cp Backlog.icns $(BUNDLE_DIR)/Contents/Resources/Backlog.icns
	@printf '%s\n' \
		'<?xml version="1.0" encoding="UTF-8"?>' \
		'<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">' \
		'<plist version="1.0">' \
		'<dict>' \
		'    <key>CFBundleExecutable</key>' \
		'    <string>backlog</string>' \
		'    <key>CFBundleIconFile</key>' \
		'    <string>Backlog.icns</string>' \
		'    <key>CFBundleIdentifier</key>' \
		'    <string>com.altenwald.backlog</string>' \
		'    <key>CFBundleName</key>' \
		'    <string>Backlog</string>' \
		'    <key>CFBundlePackageType</key>' \
		'    <string>APPL</string>' \
		'    <key>CFBundleShortVersionString</key>' \
		'    <string>$(VERSION_NUM)</string>' \
		'    <key>CFBundleVersion</key>' \
		'    <string>$(VERSION)</string>' \
		'    <key>LSMinimumSystemVersion</key>' \
		'    <string>$(MIN_MACOS_VER)</string>' \
		'    <key>NSHighResolutionCapable</key>' \
		'    <true/>' \
		'</dict>' \
		'</plist>' > $(BUNDLE_DIR)/Contents/Info.plist
	@echo "✔ Universal $(BUNDLE_DIR) created for macOS $(MIN_MACOS_VER)+!"
	@file $(BUNDLE_DIR)/Contents/MacOS/backlog

clean:
	rm -rf bin/
