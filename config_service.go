package jira4claude

// InitResult contains the result of the Init operation.
type InitResult struct {
	ConfigCreated   bool `json:"configCreated"`
	GitignoreAdded  bool `json:"gitignoreAdded"`
	GitignoreExists bool `json:"gitignoreExists"`
}

// ConfigService defines operations for managing configuration.
type ConfigService interface {
	// Init creates a new config file in the given directory.
	Init(dir, server, project string) (*InitResult, error)
}
