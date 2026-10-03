package database

import (
	"errors"
	"time"

	"OpenWAF/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func prepareInvestigationMigration(db *gorm.DB) error {
	if db.Migrator().HasTable("incidents") && !db.Migrator().HasTable(&domain.Investigation{}) {
		return db.Migrator().RenameTable("incidents", "investigations")
	}
	return nil
}

func migrateInvestigationResults(db *gorm.DB) error {
	if !db.Migrator().HasTable("findings") {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		var rows []struct {
			ID              string
			IncidentID      string
			RunID           string
			Title           string
			Summary         string
			Severity        string
			Assessment      string
			Explanation     string
			Patterns        []string                    `gorm:"serializer:json"`
			Recommendations []string                    `gorm:"serializer:json"`
			Limitations     []string                    `gorm:"serializer:json"`
			Evidence        []domain.RequestLog         `gorm:"serializer:json"`
			Trigger         domain.InvestigationTrigger `gorm:"serializer:json"`
			State           string
			CreatedAt       time.Time
		}
		query := tx.Table("findings AS f").Select("f.*").Joins("JOIN investigations AS i ON i.id = f.incident_id").Where("i.result_version = 0 OR i.result_version IS NULL").Where("f.id = (SELECT latest.id FROM findings AS latest WHERE latest.incident_id = f.incident_id ORDER BY latest.created_at DESC, latest.id DESC LIMIT 1)")
		if err := query.Find(&rows).Error; err != nil {
			return err
		}
		for _, row := range rows {
			var item domain.Investigation
			if err := tx.First(&item, "id = ?", row.IncidentID).Error; err != nil {
				return err
			}
			result := domain.InvestigationResult{Title: row.Title, Summary: row.Summary, Severity: row.Severity, Assessment: row.Assessment, Explanation: row.Explanation, Patterns: row.Patterns, Recommendations: row.Recommendations, Limitations: row.Limitations, Evidence: row.Evidence, Trigger: row.Trigger, PublishedAt: row.CreatedAt}
			if item.State == "open" && (row.State == "acknowledged" || row.State == "dismissed") {
				item.State = row.State
			}
			item.Result = &result
			item.ResultVersion = 1
			if err := tx.Save(&item).Error; err != nil {
				return err
			}
			if row.RunID != "" {
				var run domain.InvestigationRun
				if err := tx.First(&run, "id = ?", row.RunID).Error; err == nil {
					run.Result = &result
					if err := tx.Save(&run).Error; err != nil {
						return err
					}
				} else if !errors.Is(err, gorm.ErrRecordNotFound) {
					return err
				}
			}
			if tx.Migrator().HasTable("finding_reads") {
				var reads []struct {
					UserID uint
					ReadAt time.Time
				}
				if err := tx.Table("finding_reads").Where("finding_id = ?", row.ID).Find(&reads).Error; err != nil {
					return err
				}
				for _, read := range reads {
					if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.InvestigationRead{InvestigationID: item.ID, UserID: read.UserID, Version: 1, ReadAt: read.ReadAt}).Error; err != nil {
						return err
					}
				}
			}
		}

		var links []struct {
			InvestigationID string
			UserID          uint
			ConversationID  string
		}
		if err := tx.Table("investigation_follow_ups AS link").Select("execution.incident_id AS investigation_id, link.user_id, link.conversation_id").Joins("JOIN investigation_runs AS execution ON execution.id = link.run_id").Order("execution.created_at DESC, execution.id DESC").Scan(&links).Error; err != nil {
			return err
		}
		for _, link := range links {
			if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&domain.InvestigationFollowUp{InvestigationID: link.InvestigationID, UserID: link.UserID, ConversationID: link.ConversationID}).Error; err != nil {
				return err
			}
		}
		return nil
	})
}
