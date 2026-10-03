package database

import (
	"context"
	"errors"
	"time"

	"OpenWAF/internal/domain"
	"gorm.io/gorm"
)

type Store struct{ db *gorm.DB }

func NewStore(db *gorm.DB) *Store { return &Store{db: db} }

func setupCompleted(db *gorm.DB) (bool, error) {
	var user domain.User
	err := db.Select("id").Where("role = ?", "admin").Limit(1).Find(&user).Error
	return user.ID != 0, err
}

func (s *Store) SetupCompleted(ctx context.Context) (bool, error) {
	return setupCompleted(s.db.WithContext(ctx))
}

func (s *Store) CreateFirstAdmin(ctx context.Context, user *domain.User) error {
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		completed, err := setupCompleted(tx)
		if err != nil {
			return err
		}
		if completed {
			return domain.ErrSetupCompleted
		}
		return tx.Create(user).Error
	})
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrUsernameTaken
	}
	return err
}

func (s *Store) UserByUsername(ctx context.Context, username string) (domain.User, error) {
	var user domain.User
	err := s.db.WithContext(ctx).Where("username = ?", username).Limit(1).Find(&user).Error
	return user, err
}

func (s *Store) CreateSession(ctx context.Context, session *domain.UserSession, previousHash string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("expires_at <= ? OR token_hash = ?", time.Now().UTC(), previousHash).Delete(&domain.UserSession{}).Error; err != nil {
			return err
		}
		return tx.Create(session).Error
	})
}

func (s *Store) DeleteSession(ctx context.Context, hash string) error {
	return s.db.WithContext(ctx).Where("token_hash = ?", hash).Delete(&domain.UserSession{}).Error
}

func (s *Store) SessionUser(ctx context.Context, hash string, now time.Time) (domain.User, error) {
	var session domain.UserSession
	err := s.db.WithContext(ctx).Preload("User").Where("token_hash = ? AND expires_at > ?", hash, now).Limit(1).Find(&session).Error
	if err != nil {
		return domain.User{}, err
	}
	if session.ID == 0 || session.User.ID == 0 {
		return domain.User{}, domain.ErrUnauthorized
	}
	return session.User, nil
}

func (s *Store) ListServices(ctx context.Context) ([]domain.Service, error) {
	services := make([]domain.Service, 0)
	err := s.db.WithContext(ctx).Order("id").Find(&services).Error
	return services, err
}

func (s *Store) ServiceByID(ctx context.Context, id uint64) (domain.Service, error) {
	var service domain.Service
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&service).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return service, domain.ErrServiceNotFound
	}
	return service, err
}

func hostnameError(err error) error {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return domain.ErrHostnameTaken
	}
	return err
}

func (s *Store) CreateService(ctx context.Context, service *domain.Service) error {
	return hostnameError(s.db.WithContext(ctx).Create(service).Error)
}

func (s *Store) UpdateService(ctx context.Context, service *domain.Service) error {
	result := s.db.WithContext(ctx).Model(&domain.Service{}).Where("id = ?", service.ID).Select("Name", "Hostname", "UpstreamURL", "SkipTLSVerify", "Enabled", "UpdatedAt").Updates(service)
	if result.Error != nil {
		return hostnameError(result.Error)
	}
	if result.RowsAffected == 0 {
		return domain.ErrServiceNotFound
	}
	return nil
}

func (s *Store) DeleteService(ctx context.Context, service *domain.Service) error {
	return s.db.WithContext(ctx).Delete(service).Error
}

func (s *Store) EnabledService(ctx context.Context, hostname string) (domain.Service, error) {
	var service domain.Service
	err := s.db.WithContext(ctx).Where("hostname = ? AND enabled = ?", hostname, true).Limit(1).Find(&service).Error
	if err != nil {
		return service, err
	}
	if service.ID == 0 {
		return service, domain.ErrServiceNotFound
	}
	return service, nil
}
