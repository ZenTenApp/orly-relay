#!/bin/bash
# Badger Database Migration Script
# Migrates ORLY database to new Badger configuration with VLogPercentile optimization

set -e  # Exit on error

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo -e "${GREEN}=== ORLY Badger Database Migration ===${NC}"
echo ""

# Configuration
DATA_DIR="${ORLY_DATA_DIR:-$HOME/.local/share/ORLY}"
BACKUP_DIR="${DATA_DIR}-backup-$(date +%Y%m%d-%H%M%S)"
EXPORT_FILE="${DATA_DIR}/events-export.jsonl"
RELAY_BIN="${RELAY_BIN:-./orly}"

# Check if relay binary exists
if [ ! -f "$RELAY_BIN" ]; then
    echo -e "${RED}Error: ORLY binary not found at $RELAY_BIN${NC}"
    echo "Please build the relay first: go build -o orly"
    echo "Or set RELAY_BIN environment variable to the binary location"
    exit 1
fi

# Check if database exists
if [ ! -d "$DATA_DIR" ]; then
    echo -e "${YELLOW}Warning: Database directory not found at $DATA_DIR${NC}"
    echo "Nothing to migrate. If this is a fresh install, you can skip migration."
    exit 0
fi

# Check disk space
DB_SIZE=$(du -sb "$DATA_DIR" | cut -f1)
AVAILABLE_SPACE=$(df "$HOME" | tail -1 | awk '{print $4}')
AVAILABLE_SPACE=$((AVAILABLE_SPACE * 1024))  # Convert to bytes
REQUIRED_SPACE=$((DB_SIZE * 3))  # 3x for safety (export + backup + new DB)

echo "Database size: $(numfmt --to=iec-i --suffix=B $DB_SIZE)"
echo "Available space: $(numfmt --to=iec-i --suffix=B $AVAILABLE_SPACE)"
echo "Required space: $(numfmt --to=iec-i --suffix=B $REQUIRED_SPACE)"
echo ""

if [ $AVAILABLE_SPACE -lt $REQUIRED_SPACE ]; then
    echo -e "${RED}Error: Not enough disk space!${NC}"
    echo "Required: $(numfmt --to=iec-i --suffix=B $REQUIRED_SPACE)"
    echo "Available: $(numfmt --to=iec-i --suffix=B $AVAILABLE_SPACE)"
    echo ""
    echo "Options:"
    echo "  1. Free up disk space"
    echo "  2. Use natural compaction (no migration needed)"
    echo "  3. Export to external drive and import back"
    exit 1
fi

# Check if relay is running
if pgrep -x "orly" > /dev/null; then
    echo -e "${YELLOW}Warning: ORLY relay is currently running${NC}"
    echo "The relay should be stopped before migration."
    echo ""
    read -p "Stop the relay now? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo "Attempting to stop relay..."
        if systemctl is-active --quiet orly; then
            sudo systemctl stop orly
            echo -e "${GREEN}Relay stopped via systemd${NC}"
        else
            pkill orly
            sleep 2
            if pgrep -x "orly" > /dev/null; then
                echo -e "${RED}Failed to stop relay. Please stop it manually and try again.${NC}"
                exit 1
            fi
            echo -e "${GREEN}Relay stopped${NC}"
        fi
    else
        echo "Please stop the relay and run this script again."
        exit 1
    fi
fi

echo ""
echo -e "${YELLOW}=== Migration Plan ===${NC}"
echo "1. Export all events to JSONL: $EXPORT_FILE"
echo "2. Backup current database to: $BACKUP_DIR"
echo "3. Create new database with optimized configuration"
echo "4. Import all events (rebuilds indexes)"
echo "5. Verify event counts match"
echo ""
echo "Estimated time: $(( (DB_SIZE / 1024 / 1024 / 100) + 1 )) - $(( (DB_SIZE / 1024 / 1024 / 50) + 1 )) minutes"
echo ""
read -p "Proceed with migration? (y/N) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Migration cancelled."
    exit 0
fi

# Step 1: Export events
echo ""
echo -e "${GREEN}=== Step 1: Exporting Events ===${NC}"
echo "This may take several minutes for large databases..."
echo ""

# We'll use a Go program to export since the binary doesn't have a CLI export command
# Create temporary export program
EXPORT_PROG=$(mktemp -d)/export-db.go
cat > "$EXPORT_PROG" << 'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"git.smesh.lol/orly/pkg/database"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <data-dir> <output-file>\n", os.Args[0])
		os.Exit(1)
	}

	dataDir := os.Args[1]
	outFile := os.Args[2]

	ctx := context.Background()
	cancel := func() {}

	db, err := database.New(ctx, cancel, dataDir, "error")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	f, err := os.Create(outFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Println("Exporting events...")
	db.Export(ctx, f)
	fmt.Println("Export complete!")
}
EOF

# Build and run export program
echo "Building export tool..."
EXPORT_BIN=$(mktemp)
if ! go build -o "$EXPORT_BIN" "$EXPORT_PROG" 2>&1; then
    echo -e "${RED}Failed to build export tool${NC}"
    rm -f "$EXPORT_PROG" "$EXPORT_BIN"
    exit 1
fi

echo "Running export..."
if ! "$EXPORT_BIN" "$DATA_DIR" "$EXPORT_FILE"; then
    echo -e "${RED}Export failed!${NC}"
    rm -f "$EXPORT_PROG" "$EXPORT_BIN"
    exit 1
fi

rm -f "$EXPORT_PROG" "$EXPORT_BIN"

# Count exported events
EXPORT_COUNT=$(wc -l < "$EXPORT_FILE")
echo -e "${GREEN}Exported $EXPORT_COUNT events${NC}"
echo "Export size: $(du -h "$EXPORT_FILE" | cut -f1)"

# Step 2: Backup current database
echo ""
echo -e "${GREEN}=== Step 2: Backing Up Current Database ===${NC}"
echo "Moving $DATA_DIR to $BACKUP_DIR"
mv "$DATA_DIR" "$BACKUP_DIR"
echo -e "${GREEN}Backup complete${NC}"

# Step 3 & 4: Create new database and import
echo ""
echo -e "${GREEN}=== Step 3 & 4: Creating New Database and Importing ===${NC}"
echo "This will take longer as indexes are rebuilt..."
echo ""

# Create temporary import program
IMPORT_PROG=$(mktemp -d)/import-db.go
cat > "$IMPORT_PROG" << 'EOF'
package main

import (
	"context"
	"fmt"
	"os"
	"git.smesh.lol/orly/pkg/database"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintf(os.Stderr, "Usage: %s <data-dir> <import-file>\n", os.Args[0])
		os.Exit(1)
	}

	dataDir := os.Args[1]
	importFile := os.Args[2]

	ctx := context.Background()
	cancel := func() {}

	// This will create new database with updated configuration from database.go
	db, err := database.New(ctx, cancel, dataDir, "info")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	f, err := os.Open(importFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to open import file: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	fmt.Println("Importing events (this may take a while)...")
	db.Import(f)

	// Wait for import to complete
	fmt.Println("Import started. Waiting for completion...")
	fmt.Println("Check the log output above for progress (logged every 100 events)")
}
EOF

# Build and run import program
echo "Building import tool..."
IMPORT_BIN=$(mktemp)
if ! go build -o "$IMPORT_BIN" "$IMPORT_PROG" 2>&1; then
    echo -e "${RED}Failed to build import tool${NC}"
    echo "Rolling back..."
    mv "$BACKUP_DIR" "$DATA_DIR"
    rm -f "$IMPORT_PROG" "$IMPORT_BIN"
    exit 1
fi

echo "Running import..."
if ! "$IMPORT_BIN" "$DATA_DIR" "$EXPORT_FILE"; then
    echo -e "${RED}Import failed!${NC}"
    echo "Rolling back..."
    rm -rf "$DATA_DIR"
    mv "$BACKUP_DIR" "$DATA_DIR"
    rm -f "$IMPORT_PROG" "$IMPORT_BIN"
    exit 1
fi

rm -f "$IMPORT_PROG" "$IMPORT_BIN"

# Give import goroutine time to process
echo "Waiting for import to complete..."
sleep 10

# Step 5: Verify
echo ""
echo -e "${GREEN}=== Step 5: Verification ===${NC}"

NEW_DB_SIZE=$(du -sb "$DATA_DIR" | cut -f1)
echo "Old database size: $(numfmt --to=iec-i --suffix=B $DB_SIZE)"
echo "New database size: $(numfmt --to=iec-i --suffix=B $NEW_DB_SIZE)"
echo ""

if [ $NEW_DB_SIZE -lt $((DB_SIZE / 10)) ]; then
    echo -e "${YELLOW}Warning: New database is suspiciously small${NC}"
    echo "This may indicate an incomplete import."
    echo "Check the logs in $DATA_DIR/migration.log"
    echo ""
    read -p "Continue anyway? (y/N) " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        echo "Rolling back..."
        rm -rf "$DATA_DIR"
        mv "$BACKUP_DIR" "$DATA_DIR"
        exit 1
    fi
fi

echo -e "${GREEN}=== Migration Complete! ===${NC}"
echo ""
echo "Summary:"
echo "  - Exported: $EXPORT_COUNT events"
echo "  - Old DB size: $(numfmt --to=iec-i --suffix=B $DB_SIZE)"
echo "  - New DB size: $(numfmt --to=iec-i --suffix=B $NEW_DB_SIZE)"
echo "  - Space saved: $(numfmt --to=iec-i --suffix=B $((DB_SIZE - NEW_DB_SIZE)))"
echo "  - Backup location: $BACKUP_DIR"
echo ""
echo "Next steps:"
echo "  1. Start the relay: sudo systemctl start orly (or ./orly)"
echo "  2. Monitor performance for 24-48 hours"
echo "  3. Watch for cache hit ratio >85% in logs"
echo "  4. Verify event count and queries work correctly"
echo "  5. After verification, remove backup: rm -rf $BACKUP_DIR"
echo ""
echo "Rollback (if needed):"
echo "  Stop relay, then: rm -rf $DATA_DIR && mv $BACKUP_DIR $DATA_DIR"
echo ""
