package services

import (
	"context"
	"strings"

	"OpenWAF/internal/domain"
)

type Input struct {
	Name          string `json:"name" required:"true" description:"Trimmed nonblank name, at most 255 bytes"`
	Hostname      string `json:"hostname" required:"true" description:"Hostname or IP address without a port"`
	UpstreamURL   string `json:"upstreamUrl" required:"true" format:"uri" description:"HTTP or HTTPS origin, optionally with a port"`
	SkipTLSVerify bool   `json:"skipTlsVerify" default:"false"`
	Enabled       *bool  `json:"enabled" default:"true"`
}

type Store interface {
	ListServices(context.Context) ([]domain.Service, error)
	ServiceByID(context.Context, uint64) (domain.Service, error)
	CreateService(context.Context, *domain.Service) error
	UpdateService(context.Context, *domain.Service) error
	DeleteService(context.Context, *domain.Service) error
}

type Service struct{ store Store }

func New(store Store) *Service { return &Service{store: store} }

func validate(input Input) (domain.Service, error) {
	input.Name = strings.TrimSpace(input.Name)
	input.Hostname = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(input.Hostname)), ".")
	if input.Name == "" || len(input.Name) > 255 || !domain.ValidHostname(input.Hostname) {
		return domain.Service{}, domain.ValidationError("A name (up to 255 bytes) and a valid hostname or IP address without a port are required")
	}
	target, err := domain.UpstreamURL(strings.TrimSpace(input.UpstreamURL))
	if err != nil {
		return domain.Service{}, domain.ValidationError(err.Error())
	}
	enabled := true
	if input.Enabled != nil {
		enabled = *input.Enabled
	}
	return domain.Service{Name: input.Name, Hostname: input.Hostname, UpstreamURL: target.String(), SkipTLSVerify: input.SkipTLSVerify, Enabled: enabled}, nil
}

func (s *Service) List(ctx context.Context) ([]domain.Service, error) {
	return s.store.ListServices(ctx)
}

func (s *Service) Get(ctx context.Context, id uint64) (domain.Service, error) {
	return s.store.ServiceByID(ctx, id)
}

func (s *Service) Create(ctx context.Context, input Input) (domain.Service, error) {
	service, err := validate(input)
	if err != nil {
		return service, err
	}
	if err := s.store.CreateService(ctx, &service); err != nil {
		return service, err
	}
	return service, nil
}

func (s *Service) Update(ctx context.Context, existing domain.Service, input Input) (domain.Service, error) {
	service, err := validate(input)
	if err != nil {
		return service, err
	}
	service.ID, service.CreatedAt = existing.ID, existing.CreatedAt
	if err := s.store.UpdateService(ctx, &service); err != nil {
		return service, err
	}
	return service, nil
}

func (s *Service) Delete(ctx context.Context, id uint64) error {
	service, err := s.store.ServiceByID(ctx, id)
	if err != nil {
		return err
	}
	return s.store.DeleteService(ctx, &service)
}
