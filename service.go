package xcore

type Service interface {
	Start() error
	Stop() error
	Name() string
}

type serviceWrapper struct {
	service Service
	logger  *Logger
}

type ServiceManager struct {
	services []serviceWrapper
	logger   *Logger
}

func NewServiceManager(logger *Logger) *ServiceManager {
	return &ServiceManager{
		services: make([]serviceWrapper, 0),
		logger:   logger,
	}
}

func (sm *ServiceManager) Add(service Service) {
	sm.services = append(sm.services, serviceWrapper{
		service: service,
		logger:  sm.logger,
	})
	if sm.logger != nil {
		sm.logger.Info().Str("service", service.Name()).Msg("registered service")
	}
}

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

func (sm *ServiceManager) Count() int {
	return len(sm.services)
}

func (sm *ServiceManager) Services() []Service {
	services := make([]Service, len(sm.services))
	for i, s := range sm.services {
		services[i] = s.service
	}
	return services
}
