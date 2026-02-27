//go:build !(js && wasm)

package sync

import (
	"context"
	"sort"
	"sync"

	"next.orly.dev/pkg/lol/errorf"
	"next.orly.dev/pkg/database"
)

// DriverFactory is the signature for sync driver factory functions.
type DriverFactory func(ctx context.Context, db database.Database, cfg *DriverConfig) (Service, error)

// DriverConfig holds configuration for sync drivers.
type DriverConfig struct {
	// Common settings
	LogLevel string

	// Negentropy-specific settings
	TargetRelays []string
	Filter       string
	SyncInterval string
	BatchSize    int

	// Cluster-specific settings
	AdminNpubs                []string
	PropagatePrivilegedEvents bool
	PollInterval              string

	// Distributed-specific settings
	NodeID   string
	RelayURL string
	Peers    []string

	// Relay group settings
	RelayGroups []string
}

// DriverInfo contains metadata about a registered sync driver.
type DriverInfo struct {
	Name        string
	Description string
	Factory     DriverFactory
}

// Service is the interface that sync drivers must implement.
type Service interface {
	// Start begins the sync service
	Start() error
	// Stop stops the sync service
	Stop() error
	// Type returns the service type name
	Type() string
}

var (
	driversMu sync.RWMutex
	drivers   = make(map[string]*DriverInfo)
)

// RegisterDriver registers a sync driver with the given name and factory.
// This is typically called from init() in the driver package.
func RegisterDriver(name, description string, factory DriverFactory) {
	driversMu.Lock()
	defer driversMu.Unlock()
	drivers[name] = &DriverInfo{
		Name:        name,
		Description: description,
		Factory:     factory,
	}
}

// GetDriver returns the factory for the named driver, or nil if not found.
func GetDriver(name string) DriverFactory {
	driversMu.RLock()
	defer driversMu.RUnlock()
	if info, ok := drivers[name]; ok {
		return info.Factory
	}
	return nil
}

// HasDriver returns true if the named driver is registered.
func HasDriver(name string) bool {
	driversMu.RLock()
	defer driversMu.RUnlock()
	_, ok := drivers[name]
	return ok
}

// ListDrivers returns a sorted list of registered driver names.
func ListDrivers() []string {
	driversMu.RLock()
	defer driversMu.RUnlock()
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListDriversWithInfo returns information about all registered drivers.
func ListDriversWithInfo() []*DriverInfo {
	driversMu.RLock()
	defer driversMu.RUnlock()
	infos := make([]*DriverInfo, 0, len(drivers))
	for _, info := range drivers {
		infos = append(infos, info)
	}
	// Sort by name for consistent output
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].Name < infos[j].Name
	})
	return infos
}

// NewFromDriver creates a sync service using the named driver.
// Returns an error if the driver is not registered.
func NewFromDriver(ctx context.Context, driverName string, db database.Database, cfg *DriverConfig) (Service, error) {
	factory := GetDriver(driverName)
	if factory == nil {
		return nil, errorf.E("sync driver %q not available; registered: %v", driverName, ListDrivers())
	}
	return factory(ctx, db, cfg)
}
