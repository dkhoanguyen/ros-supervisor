package api

import (
	"context"

	"github.com/compose-spec/compose-go/types"
	"github.com/dkhoanguyen/ros-supervisor/models/service"
)

// ServiceProxy implements Service by delegating to implementation functions. This allows lazy init and per-method overrides
type ServiceProxy struct {
	PsFn         func(ctx context.Context, projectName string, options PsOptions) ([]ContainerSummary, error)
	interceptors []Interceptor
}

// NewServiceProxy produces a ServiceProxy
func NewServiceProxy() *ServiceProxy {
	return &ServiceProxy{}
}

// Interceptor allow to customize the compose types.Project before the actual Service method is executed
type Interceptor func(ctx context.Context, project *types.Project)

var _ Service = &ServiceProxy{}

// WithService configure proxy to use specified Service as delegate
func (s *ServiceProxy) WithService(service service.Service) *ServiceProxy {
	s.PsFn = service.Ps
	return s
}

// Ps implements Service interface
func (s *ServiceProxy) Ps(ctx context.Context, project string, options PsOptions) ([]ContainerSummary, error) {
	if s.PsFn == nil {
		return nil, ErrNotImplemented
	}
	return s.PsFn(ctx, project, options)
}
