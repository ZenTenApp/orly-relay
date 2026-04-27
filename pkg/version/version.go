package version

import (
	_ "embed"
)

//go:embed version
var V string

var Description = "relay powered by the orly framework https://git.smesh.lol/orly"

var URL = "https://git.smesh.lol/orly"
