// Command seed loads a repeatable demo dataset into the application's SQLite database.
package main

import (
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"OpenWAF/internal/database"
	"OpenWAF/internal/domain"

	"github.com/joho/godotenv"
	"gorm.io/gorm"
)

const logCount = 6000

func main() {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	path := os.Getenv("DATABASE_PATH")
	if path == "" {
		path = "data/openwaf.db"
	}
	db, err := database.Open(path)
	if err != nil {
		log.Fatal(err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal(err)
	}
	defer sqlDB.Close()
	if err := seed(db); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Seeded %d demo request logs across 5 services in %s\n", logCount, path)
}

func seed(db *gorm.DB) error {
	now := time.Now().UTC().Truncate(time.Second)
	services := []domain.Service{
		{Name: "Shop API", Hostname: "shop.demo.test", UpstreamURL: "http://127.0.0.1:9001", Enabled: true},
		{Name: "Customer Portal", Hostname: "portal.demo.test", UpstreamURL: "http://127.0.0.1:9002", Enabled: true},
		{Name: "Public Documentation", Hostname: "docs.demo.test", UpstreamURL: "http://127.0.0.1:9003", Enabled: true},
		{Name: "Legacy Admin", Hostname: "admin.demo.test", UpstreamURL: "https://127.0.0.1:9443", SkipTLSVerify: true, Enabled: true},
		{Name: "Staging Checkout", Hostname: "checkout-staging.demo.test", UpstreamURL: "http://127.0.0.1:9005", Enabled: false},
	}
	for i := range services {
		var existing domain.Service
		if err := db.Where("hostname = ?", services[i].Hostname).First(&existing).Error; err != nil && err != gorm.ErrRecordNotFound {
			return err
		} else if existing.ID != 0 {
			services[i].ID = existing.ID
			services[i].CreatedAt = existing.CreatedAt
		}
		services[i].CreatedAt, services[i].UpdatedAt = now.AddDate(0, 0, -90), now
		if services[i].ID == 0 {
			if err := db.Create(&services[i]).Error; err != nil {
				return err
			}
		} else if err := db.Save(&services[i]).Error; err != nil {
			return err
		}
	}

	ids := make([]uint, len(services))
	for i := range services {
		ids[i] = services[i].ID
	}
	return db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("service_id IN ?", ids).Delete(&domain.RequestLog{}).Error; err != nil {
			return err
		}
		if err := tx.Where("service_id IN ?", ids).Delete(&domain.Rule{}).Error; err != nil {
			return err
		}
		if err := tx.Where("service_id IN ?", ids).Delete(&domain.RulePolicy{}).Error; err != nil {
			return err
		}
		if err := tx.Where("scope = 0").Delete(&domain.RulePolicy{}).Error; err != nil {
			return err
		}
		policies := []domain.RulePolicy{
			{Scope: 0, Mode: "blocking", BlockingParanoiaLevel: 2, DetectionParanoiaLevel: 3, InboundThreshold: 5, MaxBodyBytes: 10485760, RateLimitPerMinute: 600, RateLimitAction: "block", DisabledRuleIDs: []int{}},
			{Scope: ids[0], ServiceID: &ids[0], Mode: "blocking", BlockingParanoiaLevel: 2, DetectionParanoiaLevel: 3, InboundThreshold: 5, MaxBodyBytes: 10485760, RateLimitPerMinute: 300, RateLimitAction: "block", DisabledRuleIDs: []int{}},
			{Scope: ids[1], ServiceID: &ids[1], Mode: "detection", BlockingParanoiaLevel: 1, DetectionParanoiaLevel: 2, InboundThreshold: 8, MaxBodyBytes: 5242880, RateLimitPerMinute: 500, RateLimitAction: "log", DisabledRuleIDs: []int{}},
			{Scope: ids[2], ServiceID: &ids[2], Mode: "blocking", BlockingParanoiaLevel: 1, DetectionParanoiaLevel: 2, InboundThreshold: 5, MaxBodyBytes: 2097152, RateLimitPerMinute: 900, RateLimitAction: "block", DisabledRuleIDs: []int{}},
			{Scope: ids[3], ServiceID: &ids[3], Mode: "off", BlockingParanoiaLevel: 1, DetectionParanoiaLevel: 1, InboundThreshold: 10, MaxBodyBytes: 1048576, RateLimitPerMinute: 0, RateLimitAction: "log", DisabledRuleIDs: []int{}},
		}
		for i := range policies {
			policies[i].UpdatedAt = now
			if err := tx.Create(&policies[i]).Error; err != nil {
				return err
			}
		}
		if err := tx.Create(&[]domain.Rule{
			{Name: "Protect account endpoints", Description: "Require authentication on customer account routes.", Enabled: true, Action: "block", ServiceID: &ids[1], Conditions: []domain.RuleCondition{{Target: "path", Operator: "prefix", Value: "/api/account"}}},
			{Name: "Log large exports", Description: "Record access to bulk export endpoints for review.", Enabled: true, Action: "log", ServiceID: &ids[0], Conditions: []domain.RuleCondition{{Target: "path", Operator: "contains", Value: "/export"}}},
			{Name: "Restrict legacy console", Description: "Block requests to the retired maintenance console.", Enabled: false, Action: "block", ServiceID: &ids[3], Conditions: []domain.RuleCondition{{Target: "path", Operator: "prefix", Value: "/maintenance"}}},
		}).Error; err != nil {
			return err
		}
		return tx.CreateInBatches(generateLogs(ids, services, now), 100).Error
	})
}

func generateLogs(ids []uint, services []domain.Service, now time.Time) []domain.RequestLog {
	rng := rand.New(rand.NewSource(73))
	paths := [][]string{
		{"/", "/api/products", "/api/products/sku-1042", "/api/cart", "/api/cart/items", "/api/orders", "/api/orders/ORD-20391", "/api/search?q=running+shoes", "/assets/app.js", "/health"},
		{"/", "/login", "/api/account/profile", "/api/account/orders", "/api/account/preferences", "/assets/site.css", "/forgot-password", "/api/session/refresh"},
		{"/", "/guides/getting-started", "/api/search?q=rules", "/reference/proxy", "/reference/policies", "/assets/docs.css", "/robots.txt"},
		{"/login", "/admin", "/admin/users", "/api/internal/status", "/maintenance/health", "/assets/admin.js"},
		{"/checkout", "/api/checkout/session", "/api/payment/confirm", "/health"},
	}
	methods := []string{"GET", "GET", "GET", "POST", "GET", "PUT", "DELETE", "OPTIONS"}
	ips := []string{"198.51.100.14", "198.51.100.27", "203.0.113.42", "203.0.113.81", "192.0.2.19", "192.0.2.88", "45.129.14.22", "91.198.174.12", "185.220.101.4", "104.21.54.18", "172.67.142.77", "81.2.69.142", "8.8.8.8", "37.120.212.8", "146.70.112.19", "5.188.206.14", "193.32.162.55", "103.152.220.9"}
	countries := []string{"US", "US", "PL", "GB", "DE", "FR", "NL", "SE", "CA", "AU", "BR", "SG"}
	attacks := []struct{ path, id, reason, signal string }{
		{"/search?q=%27%20OR%201%3D1--", "942100", "SQL injection attempt", "sqli"},
		{"/api/products?id=1%20UNION%20SELECT%20password", "942430", "SQL UNION injection", "sqli"},
		{"/redirect?url=http://169.254.169.254/latest/meta-data", "918274", "Server-side request forgery probe", "ssrf"},
		{"/../../etc/passwd", "930100", "Path traversal attempt", "traversal"},
		{"/search?q=%3Cscript%3Ealert(1)%3C/script%3E", "941100", "Cross-site scripting attempt", "xss"},
		{"/api/login?user=admin%27--", "942100", "SQL injection in login request", "sqli"},
		{"/wp-login.php", "913100", "Suspicious scanner path", "scanner"},
		{"/.env", "930120", "Sensitive file access attempt", "exposure"},
		{"/cgi-bin/.%2e/.%2e/etc/passwd", "930100", "Encoded path traversal", "traversal"},
	}
	logs := make([]domain.RequestLog, 0, logCount)
	for i := 0; i < logCount; i++ {
		svc := rng.Intn(5)
		stamp := now.Add(-time.Duration(rng.Intn(30*24*60)) * time.Minute).Add(-time.Duration(rng.Intn(60)) * time.Second)
		ipIndex := rng.Intn(len(ips))
		method := methods[rng.Intn(len(methods))]
		path := paths[svc][rng.Intn(len(paths[svc]))]
		action, ruleID, reason, status, errCat := "allowed", "", "", 200, ""
		var matches []domain.RuleMatch
		var signals []string
		// Attacks cluster by source IP and are more common on public facing services.
		attackChance := 0.045
		if svc == 3 {
			attackChance = 0.09
		}
		if rng.Float64() < attackChance {
			attack := attacks[rng.Intn(len(attacks))]
			path, method, action, ruleID, reason, status = attack.path, "GET", "blocked", attack.id, attack.reason, 403
			// Keep a few repeat offenders visible in the blocked-sources summary.
			ipIndex = []int{7, 8, 13, 15, 16}[rng.Intn(5)]
			signals = []string{attack.signal}
			matches = []domain.RuleMatch{{RuleID: ruleID, Source: "REQUEST_URI", Message: reason, Severity: "CRITICAL", Tags: []string{"attack", attack.signal}}}
		} else if rng.Float64() < 0.018 {
			status = []int{500, 502, 503}[rng.Intn(3)]
			errCat = "upstream"
			reason = "Upstream service returned an error"
		} else if rng.Float64() < 0.025 {
			status, reason = 404, "Route not found"
		} else if svc == 1 && path == "/api/account/profile" && rng.Float64() < 0.07 {
			status, reason = 401, "Authentication required"
		}
		country := countries[rng.Intn(len(countries))]
		serviceID := ids[svc]
		requestBytes := int64(250 + rng.Intn(7000))
		responseBytes := int64(300 + rng.Intn(42000))
		if method == "POST" || method == "PUT" {
			requestBytes += int64(rng.Intn(15000))
		}
		if status >= 500 {
			responseBytes = int64(80 + rng.Intn(700))
		}
		logs = append(logs, domain.RequestLog{Timestamp: stamp, ServiceID: &serviceID, Hostname: services[svc].Hostname, IP: ips[ipIndex], CountryCode: &country, Method: method, Path: path, Action: action, RuleID: ruleID, Reason: reason, Status: status, ErrorCategory: errCat, DurationMs: float64(8+rng.Intn(220)) + rng.Float64(), RequestBytes: requestBytes, ResponseBytes: responseBytes, RuleMatches: matches, SecuritySignals: signals})
	}
	return logs
}
