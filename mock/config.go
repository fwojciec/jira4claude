package mock

import "github.com/fwojciec/jira4claude"

// Compile-time interface verification.
var _ jira4claude.ConfigService = (*ConfigService)(nil)

// ConfigService is a mock implementation of jira4claude.ConfigService.
// Each method delegates to its corresponding function field (e.g., Init calls InitFn).
// Calling a method without setting its function field will panic.
type ConfigService struct {
	InitFn func(dir, server, project string) (*jira4claude.InitResult, error)
}

func (s *ConfigService) Init(dir, server, project string) (*jira4claude.InitResult, error) {
	return s.InitFn(dir, server, project)
}
