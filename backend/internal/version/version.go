package version

// version is injected at build time via -ldflags -X
var version = "dev"

// Get returns the embedded application build version.
func Get() string {
	return version
}
