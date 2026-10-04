package telemetry

import (
	"context"
	"testing"

	"OpenWAF/internal/domain"
)

type recordingStore struct {
	Store
	event domain.RequestLog
}

func (s *recordingStore) SaveRequest(_ context.Context, event *domain.RequestLog) error {
	s.event = *event
	return nil
}

type countryLookupFunc func(string) *string

func (f countryLookupFunc) CountryCode(ip string) *string { return f(ip) }

func TestRecordPersistsResolvedCountry(t *testing.T) {
	store := &recordingStore{}
	country := "US"
	service := New(store, countryLookupFunc(func(ip string) *string {
		if ip == "8.8.8.8" {
			return &country
		}
		return nil
	}))
	for _, ip := range []string{"8.8.8.8", "10.0.0.1"} {
		event := domain.RequestLog{IP: ip}
		if err := service.Record(context.Background(), &event); err != nil {
			t.Fatal(err)
		}
		if ip == "8.8.8.8" {
			if store.event.CountryCode == nil || *store.event.CountryCode != "US" {
				t.Fatal("country not persisted")
			}
		} else if store.event.CountryCode != nil {
			t.Fatal("private IP has country")
		}
	}
}
