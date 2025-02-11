package main

// Constants for type indicators
const (
	TYPE_INDICATOR_ARTIFACT_GROUP         = "ARTIFACT_GROUP"
	TYPE_INDICATOR_COMMAND                = "COMMAND"
	TYPE_INDICATOR_DIRECTORY              = "DIRECTORY" // deprecated, use PATH instead.
	TYPE_INDICATOR_FILE                   = "FILE"
	TYPE_INDICATOR_PATH                   = "PATH"
	TYPE_INDICATOR_WINDOWS_REGISTRY_KEY   = "REGISTRY_KEY"
	TYPE_INDICATOR_WINDOWS_REGISTRY_VALUE = "REGISTRY_VALUE"
	TYPE_INDICATOR_WMI_QUERY              = "WMI"
)

// Constants for supported OS
const (
	SUPPORTED_OS_ANDROID = "Android"
	SUPPORTED_OS_DARWIN  = "Darwin"
	SUPPORTED_OS_ESXI    = "ESXi"
	SUPPORTED_OS_IOS     = "iOS"
	SUPPORTED_OS_LINUX   = "Linux"
	SUPPORTED_OS_WINDOWS = "Windows"
)

// SupportedOS is a set of supported operating systems.
var SUPPORTED_OS = map[string]bool{
	SUPPORTED_OS_ANDROID: true,
	SUPPORTED_OS_DARWIN:  true,
	SUPPORTED_OS_ESXI:    true,
	SUPPORTED_OS_IOS:     true,
	SUPPORTED_OS_LINUX:   true,
	SUPPORTED_OS_WINDOWS: true,
}

// TopLevelKeys is a set of keys used in the top level of configuration.
var TOP_LEVEL_KEYS = map[string]bool{
	"aliases":      true,
	"conditions":   true, // deprecated as of version 20220710.
	"doc":          true,
	"labels":       true, // deprecated as of version 20220311.
	"name":         true,
	"provides":     true, // deprecated as of version 20240210.
	"sources":      true,
	"supported_os": true,
	"urls":         true,
}
