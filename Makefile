.PHONY: build test bundle clean

APP_NAME = Backlog
BUNDLE_DIR = bin/$(APP_NAME).app

build:
	go build -o bin/backlog ./cmd/backlog

test:
	go test -v ./...

bundle:
	@echo "Building Universal binary (arm64 + amd64)..."
	@mkdir -p bin
	@CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 go build -o bin/backlog_arm64 ./cmd/backlog
	@CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 go build -o bin/backlog_amd64 ./cmd/backlog
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
		'    <string>1.0.0</string>' \
		'    <key>LSMinimumSystemVersion</key>' \
		'    <string>11.0</string>' \
		'    <key>NSHighResolutionCapable</key>' \
		'    <true/>' \
		'</dict>' \
		'</plist>' > $(BUNDLE_DIR)/Contents/Info.plist
	@echo "✔ Universal $(BUNDLE_DIR) created! (Apple Silicon + Intel)"
	@file $(BUNDLE_DIR)/Contents/MacOS/backlog

clean:
	rm -rf bin/
