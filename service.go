// Package xcore provides service management for the xcore framework.
//
// This package defines the Service interface and ServiceManager for managing
// application services. Services are components that can be started and stopped
// as part of the application lifecycle.
package xcore

// Service defines the interface for application services.
// Implement this interface to create custom services that can be managed by the framework.
type Service interface {
	Start() error
	Stop() error
	Name() string
}

// serviceWrapper wraps a Service with its logger for internal tracking.
type serviceWrapper struct {
	service Service
	logger  *Logger
}

// ServiceManager manages the lifecycle of application services.
// It starts services in order and stops them in reverse order during shutdown.
type ServiceManager struct {
	services []serviceWrapper
	logger   *Logger
}

// NewServiceManager creates a new ServiceManager with an optional logger.
func NewServiceManager(logger *Logger) *ServiceManager {
	return &ServiceManager{
		services: make([]serviceWrapper, 0),
		logger:   logger,
	}
}

// Add registers a service with the service manager.
// The service will be started when StartAll is called.
func (sm *ServiceManager) Add(service Service) {
	sm.services = append(sm.services, serviceWrapper{
		service: service,
		logger:  sm.logger,
	})
	if sm.logger != nil {
		sm.logger.Info().Str("service", service.Name()).Msg("registered service")
	}
}

// StartAll starts all registered services in order.
// Returns an error if any service fails to start.
func (sm *ServiceManager) StartAll() error {
	for _, s := range sm.services {
		if sm.logger != nil {
			sm.logger.Info().Str("service", s.service.Name()).Msg("starting service")
		}
		if err := s.service.Start(); err != nil {
			if sm.logger != nil {
				sm.logger.Error().Err(err).Str("service", s.service.Name()).Msg("failed to start service")
			}
			return err
		}
		if sm.logger != nil {
			sm.logger.Info().Str("service", s.service.Name()).Msg("service started")
		}
	}
	return nil
}

// StopAll stops all registered services in reverse order.
// Logs errors but continues stopping remaining services.
func (sm *ServiceManager) StopAll() {
	for i := len(sm.services) - 1; i >= 0; i-- {
		s := sm.services[i]
		if sm.logger != nil {
			sm.logger.Info().Str("service", s.service.Name()).Msg("stopping service")
		}
		if err := s.service.Stop(); err != nil {
			if sm.logger != nil {
				sm.logger.Error().Err(err).Str("service", s.service.Name()).Msg("failed to stop service")
			}
		} else {
			if sm.logger != nil {
				sm.logger.Info().Str("service", s.service.Name()).Msg("service stopped")
			}
		}
	}
}

// Count returns the number of registered services.
func (sm *ServiceManager) Count() int {
	return len(sm.services)
}

// Services returns a slice of all registered services.
func (sm *ServiceManager) Services() []Service {
	services := make([]Service, len(sm.services))
	for i, s := range sm.services {
		services[i] = s.service
	}
	return services
}
