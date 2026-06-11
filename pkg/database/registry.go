//go:build !(js && wasm)

package database

import (
	"context"
	"sort"

	"git.smesh.lol/orly/pkg/lol/errorf"
)

// DriverFactory is the signature for database driver factory functions.
type DriverFactory func(ctx context.Context, cancel context.CancelFunc, cfg *DatabaseConfig) (Database, error)

// DriverInfo contains metadata about a registered driver.
type DriverInfo struct {
	Name        string
	Description string
	Factory     DriverFactory
}

// drivers is populated during init() (single-threaded) and only read after main starts.
var drivers = make(map[string]*DriverInfo)

// RegisterDriver registers a database driver with the given name and factory.
// This is typically called from init() in the driver package.
func RegisterDriver(name, description string, factory DriverFactory) {
	drivers[name] = &DriverInfo{
		Name:        name,
		Description: description,
		Factory:     factory,
	}
}

// GetDriver returns the factory for the named driver, or nil if not found.
func GetDriver(name string) DriverFactory {
	if info, ok := drivers[name]; ok {
		return info.Factory
	}
	return nil
}

// HasDriver returns true if the named driver is registered.
func HasDriver(name string) bool {
	_, ok := drivers[name]
	return ok
}

// ListDrivers returns a sorted list of registered driver names.
func ListDrivers() []string {
	names := make([]string, 0, len(drivers))
	for name := range drivers {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// ListDriversWithInfo returns information about all registered drivers.
func ListDriversWithInfo() []*DriverInfo {
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

// NewFromDriver creates a database using the named driver.
// Returns an error if the driver is not registered.
func NewFromDriver(ctx context.Context, cancel context.CancelFunc, driverName string, cfg *DatabaseConfig) (Database, error) {
	factory := GetDriver(driverName)
	if factory == nil {
		return nil, errorf.E("database driver %q not available; registered: %v", driverName, ListDrivers())
	}
	return factory(ctx, cancel, cfg)
}
