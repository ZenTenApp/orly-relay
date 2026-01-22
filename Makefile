# ORLY Nostr Relay Build System
.PHONY: all orly orly-db orly-acl orly-launcher proto clean test deploy web install help
.PHONY: orly-db-badger orly-db-neo4j orly-acl-follows orly-acl-managed orly-acl-curation
.PHONY: all-split arm64-split install-split

# Build flags
CGO_ENABLED ?= 0
GOOS ?= $(shell go env GOOS)
GOARCH ?= $(shell go env GOARCH)
BUILD_FLAGS = CGO_ENABLED=$(CGO_ENABLED) GOOS=$(GOOS) GOARCH=$(GOARCH)

# Binaries
BIN_DIR = .
ORLY = $(BIN_DIR)/orly
ORLY_LAUNCHER = $(BIN_DIR)/orly-launcher

# Legacy monolithic binaries (for backwards compatibility)
ORLY_DB = $(BIN_DIR)/orly-db
ORLY_ACL = $(BIN_DIR)/orly-acl

# Split database backends
ORLY_DB_BADGER = $(BIN_DIR)/orly-db-badger
ORLY_DB_NEO4J = $(BIN_DIR)/orly-db-neo4j

# Split ACL modes
ORLY_ACL_FOLLOWS = $(BIN_DIR)/orly-acl-follows
ORLY_ACL_MANAGED = $(BIN_DIR)/orly-acl-managed
ORLY_ACL_CURATION = $(BIN_DIR)/orly-acl-curation

# === Default Targets (Legacy) ===

# Default target: build everything (legacy monolithic)
all: orly orly-db orly-launcher
	@echo "All binaries built successfully"

# Build everything including ACL (when proto exists)
all-acl: proto orly orly-db orly-acl orly-launcher
	@echo "All binaries (including ACL) built successfully"

# === Split Binaries (New) ===

# Build split mode: orly + orly-db-badger + orly-acl-follows + launcher
all-split: proto orly orly-db-badger orly-acl-follows orly-launcher
	@echo "Split mode binaries built successfully"

# Build all split binaries (all backends and all ACL modes)
all-backends: proto orly orly-db-badger orly-db-neo4j orly-acl-follows orly-acl-managed orly-acl-curation orly-launcher
	@echo "All backend and ACL mode binaries built successfully"

# Main relay binary
orly:
	$(BUILD_FLAGS) go build -o $(ORLY) .

# === Database Backends ===

# Legacy monolithic database server
orly-db:
	$(BUILD_FLAGS) go build -o $(ORLY_DB) ./cmd/orly-db

# Badger database server
orly-db-badger:
	$(BUILD_FLAGS) go build -o $(ORLY_DB_BADGER) ./cmd/orly-db-badger

# Neo4j database server
orly-db-neo4j:
	$(BUILD_FLAGS) go build -o $(ORLY_DB_NEO4J) ./cmd/orly-db-neo4j

# === ACL Modes ===

# Legacy monolithic ACL server (requires proto generation first)
orly-acl:
	$(BUILD_FLAGS) go build -o $(ORLY_ACL) ./cmd/orly-acl

# Follows ACL server (whitelist based on admin follows)
orly-acl-follows:
	$(BUILD_FLAGS) go build -o $(ORLY_ACL_FOLLOWS) ./cmd/orly-acl-follows

# Managed ACL server (NIP-86 fine-grained control)
orly-acl-managed:
	$(BUILD_FLAGS) go build -o $(ORLY_ACL_MANAGED) ./cmd/orly-acl-managed

# Curation ACL server (rate-limited trust tiers)
orly-acl-curation:
	$(BUILD_FLAGS) go build -o $(ORLY_ACL_CURATION) ./cmd/orly-acl-curation

# Process supervisor/launcher
orly-launcher:
	$(BUILD_FLAGS) go build -o $(ORLY_LAUNCHER) ./cmd/orly-launcher

# Generate protobuf code
proto:
	cd proto && buf generate

# === Cross-Compile for ARM64 ===

# Build for ARM64 (relay.orly.dev deployment) - legacy
arm64:
	$(MAKE) GOOS=linux GOARCH=arm64 all

# Build everything for ARM64 including ACL - legacy
arm64-acl:
	$(MAKE) GOOS=linux GOARCH=arm64 all-acl

# Build split mode for ARM64 (recommended for relay.orly.dev)
arm64-split:
	$(MAKE) GOOS=linux GOARCH=arm64 all-split

# Build all backends for ARM64
arm64-backends:
	$(MAKE) GOOS=linux GOARCH=arm64 all-backends

# === Other Targets ===

# Build web UI and embed
web:
	./scripts/update-embedded-web.sh

# Clean build artifacts
clean:
	rm -f $(ORLY) $(ORLY_DB) $(ORLY_ACL) $(ORLY_LAUNCHER)
	rm -f $(ORLY_DB_BADGER) $(ORLY_DB_NEO4J)
	rm -f $(ORLY_ACL_FOLLOWS) $(ORLY_ACL_MANAGED) $(ORLY_ACL_CURATION)
	rm -f orly-db-arm64 orly-acl-arm64 orly-launcher-arm64 next.orly.dev

# Run tests
test:
	./scripts/test.sh

# Deploy to relay.orly.dev (builds on remote) - legacy
deploy:
	ssh relay.orly.dev 'cd ~/src/next.orly.dev && git pull origin main && make all && sudo /usr/sbin/setcap cap_net_bind_service=+ep ~/.local/bin/next.orly.dev && sudo systemctl restart orly'

# Deploy with ACL server - legacy
deploy-acl:
	ssh relay.orly.dev 'cd ~/src/next.orly.dev && git pull origin main && make all-acl && sudo /usr/sbin/setcap cap_net_bind_service=+ep ~/.local/bin/next.orly.dev && sudo systemctl restart orly'

# Deploy split mode (recommended)
deploy-split:
	ssh relay.orly.dev 'cd ~/src/next.orly.dev && git pull origin main && make all-split && sudo /usr/sbin/setcap cap_net_bind_service=+ep ~/.local/bin/next.orly.dev && sudo systemctl restart orly'

# Install all binaries locally - legacy
install: all
	mkdir -p ~/.local/bin
	cp $(ORLY) $(ORLY_DB) $(ORLY_LAUNCHER) ~/.local/bin/

# Install including ACL - legacy
install-acl: all-acl
	mkdir -p ~/.local/bin
	cp $(ORLY) $(ORLY_DB) $(ORLY_ACL) $(ORLY_LAUNCHER) ~/.local/bin/

# Install split mode binaries
install-split: all-split
	mkdir -p ~/.local/bin
	cp $(ORLY) $(ORLY_DB_BADGER) $(ORLY_ACL_FOLLOWS) $(ORLY_LAUNCHER) ~/.local/bin/

# Install all backends and modes
install-backends: all-backends
	mkdir -p ~/.local/bin
	cp $(ORLY) $(ORLY_DB_BADGER) $(ORLY_DB_NEO4J) $(ORLY_ACL_FOLLOWS) $(ORLY_ACL_MANAGED) $(ORLY_ACL_CURATION) $(ORLY_LAUNCHER) ~/.local/bin/

# Help
help:
	@echo "ORLY Build Targets:"
	@echo ""
	@echo "  Split Mode (Recommended):"
	@echo "    all-split      - Build orly + orly-db-badger + orly-acl-follows + launcher"
	@echo "    all-backends   - Build all database backends and ACL modes"
	@echo "    arm64-split    - Cross-compile split mode for ARM64"
	@echo "    deploy-split   - Deploy split mode to relay.orly.dev"
	@echo "    install-split  - Install split mode to ~/.local/bin/"
	@echo ""
	@echo "  Database Backends:"
	@echo "    orly-db-badger - Build Badger database server"
	@echo "    orly-db-neo4j  - Build Neo4j database server"
	@echo ""
	@echo "  ACL Modes:"
	@echo "    orly-acl-follows  - Build Follows ACL server (whitelist)"
	@echo "    orly-acl-managed  - Build Managed ACL server (NIP-86)"
	@echo "    orly-acl-curation - Build Curation ACL server (rate limits)"
	@echo ""
	@echo "  Legacy (Monolithic):"
	@echo "    all            - Build orly + orly-db + launcher"
	@echo "    all-acl        - Build all including orly-acl"
	@echo "    orly-db        - Build monolithic database server"
	@echo "    orly-acl       - Build monolithic ACL server"
	@echo ""
	@echo "  Core:"
	@echo "    orly           - Build main relay binary"
	@echo "    orly-launcher  - Build process supervisor"
	@echo "    proto          - Generate protobuf code"
	@echo "    web            - Rebuild embedded web UI"
	@echo "    test           - Run test suite"
	@echo "    clean          - Remove build artifacts"
	@echo "    help           - Show this help"
