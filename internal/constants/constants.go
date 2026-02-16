package constants

import "path/filepath"

var (
	// Version is the version as described by git.
	Version string
)

// Installation directory prefix and paths. Values have hard-coded defaults but
// can be changed at compile time by overriding the variable with an ldflag.
var (
	PrefixDir     string
	LibDir        string
	SysconfDir    string
	LocalStateDir string
	DataDir       string

	// ConfigDir is a path to a location where configuration data is assumed to
	// be stored.
	ConfigDir string

	// StateDir is a path to a location where local state information can be
	// stored.
	StateDir string

	// CacheDir is a path to a location where cache data can be stored.
	CacheDir string

	// PlaybookInProgressMarker is a path to a marker file that is created to communicate a playbook execution in progress,
	// and deleted when a playbook is no longer in progress.
	PlaybookInProgressMarker string
)

func init() {
	if PrefixDir == "" {
		PrefixDir = filepath.Join("/", "usr", "local")
	}

	if LibDir == "" {
		LibDir = filepath.Join(PrefixDir, "lib")
	}

	if SysconfDir == "" {
		SysconfDir = filepath.Join(PrefixDir, "etc")
	}

	if LocalStateDir == "" {
		LocalStateDir = filepath.Join(PrefixDir, "var")
	}

	if DataDir == "" {
		DataDir = filepath.Join(PrefixDir, "share")
	}

	if ConfigDir == "" {
		ConfigDir = filepath.Join(SysconfDir, "rhc-worker-playbook")
	}

	if StateDir == "" {
		StateDir = filepath.Join(LocalStateDir, "lib", "rhc-worker-playbook")
	}

	if CacheDir == "" {
		CacheDir = filepath.Join(LocalStateDir, "cache", "rhc-worker-playbook")
	}

	PlaybookInProgressMarker = filepath.Join(StateDir, ".playbook-in-progress")
}
