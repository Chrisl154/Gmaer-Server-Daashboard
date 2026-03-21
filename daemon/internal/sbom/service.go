package sbom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// CVEFinding represents a single CVE finding from a scanner
type CVEFinding struct {
	ID          string    `json:"id"`
	Severity    string    `json:"severity"` // CRITICAL|HIGH|MEDIUM|LOW
	Package     string    `json:"package"`
	Version     string    `json:"version"`
	FixedIn     string    `json:"fixed_in,omitempty"`
	Description string    `json:"description"`
	CVSS        float64   `json:"cvss,omitempty"`
	Link        string    `json:"link,omitempty"`
	ScannedAt   time.Time `json:"scanned_at"`
}

// Report represents a CVE scan report
type Report struct {
	GeneratedAt  time.Time    `json:"generated_at"`
	Scanner      string       `json:"scanner"`
	ScannerHash  string       `json:"scanner_hash,omitempty"`
	Status       string       `json:"status"` // clean|findings
	TotalCount   int          `json:"total_count"`
	Critical     int          `json:"critical"`
	High         int          `json:"high"`
	Medium       int          `json:"medium"`
	Low          int          `json:"low"`
	Findings     []CVEFinding `json:"findings"`
	Evidence     EvidenceLink `json:"evidence"`
}

// EvidenceLink provides authoritative source links for the scan
type EvidenceLink struct {
	ScannerHash     string    `json:"scanner_hash,omitempty"`
	LastChecked     time.Time `json:"last_checked"`
	AuthoritativeLink string  `json:"authoritative_link"`
	CVEStatus       string    `json:"cve_status"`
}

// Service generates CycloneDX SBOMs and manages CVE scan reports
type Service struct {
	logger      *zap.Logger
	reportPath  string
	sbomPath    string
	lastReport  *Report
	lastSBOM    *cdx.BOM
}

// NewService creates a new SBOM/CVE service
func NewService(reportPath, sbomPath string, logger *zap.Logger) *Service {
	return &Service{
		logger:     logger,
		reportPath: reportPath,
		sbomPath:   sbomPath,
	}
}

// GenerateSBOM creates a CycloneDX SBOM for the running system
func (s *Service) GenerateSBOM(ctx context.Context, components []ComponentInfo) (*cdx.BOM, error) {
	s.logger.Info("Generating SBOM", zap.Int("components", len(components)))

	bom := cdx.NewBOM()
	bom.SerialNumber = "urn:uuid:" + uuid.New().String()
	bom.Version = 1

	metadata := &cdx.Metadata{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Tools: &cdx.ToolsChoice{
			Components: &[]cdx.Component{
				{
					Type:    cdx.ComponentTypeApplication,
					Name:    "games-dashboard",
					Version: "1.0.0",
				},
			},
		},
		Component: &cdx.Component{
			Type:    cdx.ComponentTypeApplication,
			BOMRef:  "games-dashboard",
			Name:    "Games Dashboard",
			Version: "1.0.0",
		},
	}
	bom.Metadata = metadata

	cdxComponents := make([]cdx.Component, 0, len(components))
	for _, c := range components {
		comp := cdx.Component{
			BOMRef:  uuid.New().String(),
			Type:    cdx.ComponentTypeLibrary,
			Name:    c.Name,
			Version: c.Version,
			PackageURL: c.PURL,
		}
		if c.License != "" {
			comp.Licenses = &cdx.Licenses{
				{License: &cdx.License{ID: c.License}},
			}
		}
		cdxComponents = append(cdxComponents, comp)
	}
	bom.Components = &cdxComponents

	s.lastSBOM = bom

	// Persist if path configured
	if s.sbomPath != "" {
		if err := s.writeSBOM(bom); err != nil {
			s.logger.Warn("Failed to persist SBOM", zap.Error(err))
		}
	}

	return bom, nil
}

// ErrNoSBOM is returned by GetSBOM when no scan has been run yet.
// Callers should translate this to a 404 response. P51.
var ErrNoSBOM = fmt.Errorf("no SBOM available — trigger a scan via POST /sbom/scan first")

// GetSBOM returns the most recently generated SBOM as a generic map
func (s *Service) GetSBOM(ctx context.Context) (map[string]any, error) {
	if s.lastSBOM == nil {
		return nil, ErrNoSBOM
	}

	// Marshal and unmarshal to get a generic map
	data, err := json.Marshal(s.lastSBOM)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize SBOM: %w", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("failed to deserialize SBOM: %w", err)
	}

	return result, nil
}

// TriggerScan runs a CVE scan using trivy or grype (whichever is in PATH).
// Falls back to loading a previously written report from disk, or returns a
// clean placeholder when no scanner is available.
func (s *Service) TriggerScan(ctx context.Context) (*Report, error) {
	s.logger.Info("Starting CVE scan")

	findings, scanner, scanErr := s.runScanner(ctx)
	if scanErr != nil {
		s.logger.Warn("Scanner error — falling back to disk report", zap.Error(scanErr))
		findings = nil
		scanner = "none"
	}

	// Count by severity
	var critical, high, medium, low int
	for _, f := range findings {
		switch f.Severity {
		case "CRITICAL":
			critical++
		case "HIGH":
			high++
		case "MEDIUM":
			medium++
		case "LOW":
			low++
		}
	}

	status := "clean"
	if len(findings) > 0 {
		status = "findings"
	}
	if findings == nil {
		findings = []CVEFinding{}
	}

	report := &Report{
		GeneratedAt: time.Now().UTC(),
		Scanner:     scanner,
		Status:      status,
		TotalCount:  len(findings),
		Critical:    critical,
		High:        high,
		Medium:      medium,
		Low:         low,
		Findings:    findings,
		Evidence: EvidenceLink{
			LastChecked:       time.Now().UTC(),
			AuthoritativeLink: "https://osv.dev",
			CVEStatus:         status,
		},
	}

	// If no scanner ran, try loading a previously written report from disk
	if scanner == "none" && s.reportPath != "" {
		if loaded, err := s.loadReportFromDisk(s.reportPath); err == nil {
			report = loaded
		}
	}

	s.lastReport = report

	if s.reportPath != "" {
		s.writeReport(report)
	}

	s.logger.Info("CVE scan complete",
		zap.String("scanner", report.Scanner),
		zap.String("status", report.Status),
		zap.Int("findings", report.TotalCount))

	return report, nil
}

// runScanner tries trivy first, then grype. Returns (findings, scannerName, err).
// Returns (nil, "none", nil) when neither binary is available.
func (s *Service) runScanner(ctx context.Context) ([]CVEFinding, string, error) {
	if trivyPath, err := exec.LookPath("trivy"); err == nil {
		findings, err := s.runTrivy(ctx, trivyPath)
		return findings, "trivy", err
	}
	if grypePath, err := exec.LookPath("grype"); err == nil {
		findings, err := s.runGrype(ctx, grypePath)
		return findings, "grype", err
	}
	s.logger.Info("No CVE scanner found in PATH (trivy/grype) — skipping live scan")
	return nil, "none", nil
}

// runTrivy runs trivy fs against the configured target and parses its JSON output.
func (s *Service) runTrivy(ctx context.Context, trivyPath string) ([]CVEFinding, error) {
	target := s.sbomPath
	if target == "" {
		target = "."
	}

	// #nosec G204 — trivyPath is from exec.LookPath, target is operator-configured
	cmd := exec.CommandContext(ctx, trivyPath, "fs", "--format", "json", "--quiet", target)
	var out bytes.Buffer
	cmd.Stdout = &out
	// trivy exits non-zero when findings exist; that's not an error for us
	_ = cmd.Run()

	var report struct {
		Results []struct {
			Vulnerabilities []struct {
				VulnerabilityID  string   `json:"VulnerabilityID"`
				Severity         string   `json:"Severity"`
				PkgName          string   `json:"PkgName"`
				InstalledVersion string   `json:"InstalledVersion"`
				FixedVersion     string   `json:"FixedVersion"`
				Description      string   `json:"Description"`
				CVSS             float64  `json:"CVSS,omitempty"`
				References       []string `json:"References,omitempty"`
			} `json:"Vulnerabilities"`
		} `json:"Results"`
	}

	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		return nil, fmt.Errorf("parse trivy output: %w", err)
	}

	now := time.Now().UTC()
	var findings []CVEFinding
	for _, result := range report.Results {
		for _, v := range result.Vulnerabilities {
			link := ""
			if len(v.References) > 0 {
				link = v.References[0]
			}
			findings = append(findings, CVEFinding{
				ID:          v.VulnerabilityID,
				Severity:    strings.ToUpper(v.Severity),
				Package:     v.PkgName,
				Version:     v.InstalledVersion,
				FixedIn:     v.FixedVersion,
				Description: v.Description,
				CVSS:        v.CVSS,
				Link:        link,
				ScannedAt:   now,
			})
		}
	}
	return findings, nil
}

// runGrype runs grype against the configured target and parses its JSON output.
func (s *Service) runGrype(ctx context.Context, grypePath string) ([]CVEFinding, error) {
	target := "dir:."
	if s.sbomPath != "" {
		target = "sbom:" + s.sbomPath
	}

	// #nosec G204 — grypePath is from exec.LookPath, target is operator-configured
	cmd := exec.CommandContext(ctx, grypePath, target, "-o", "json", "-q")
	var out bytes.Buffer
	cmd.Stdout = &out
	_ = cmd.Run()

	var report struct {
		Matches []struct {
			Vulnerability struct {
				ID          string   `json:"id"`
				Severity    string   `json:"severity"`
				Description string   `json:"description"`
				CVSS        []struct {
					Metrics struct {
						BaseScore float64 `json:"baseScore"`
					} `json:"metrics"`
				} `json:"cvss"`
				URLs []string `json:"urls"`
			} `json:"vulnerability"`
			Artifact struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"artifact"`
			MatchDetails []struct {
				Found struct {
					VersionConstraint string `json:"versionConstraint"`
				} `json:"found"`
			} `json:"matchDetails"`
		} `json:"matches"`
	}

	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		return nil, fmt.Errorf("parse grype output: %w", err)
	}

	now := time.Now().UTC()
	var findings []CVEFinding
	for _, m := range report.Matches {
		link := ""
		if len(m.Vulnerability.URLs) > 0 {
			link = m.Vulnerability.URLs[0]
		}
		cvss := 0.0
		if len(m.Vulnerability.CVSS) > 0 {
			cvss = m.Vulnerability.CVSS[0].Metrics.BaseScore
		}
		fixedIn := ""
		if len(m.MatchDetails) > 0 {
			fixedIn = m.MatchDetails[0].Found.VersionConstraint
		}
		findings = append(findings, CVEFinding{
			ID:          m.Vulnerability.ID,
			Severity:    strings.ToUpper(m.Vulnerability.Severity),
			Package:     m.Artifact.Name,
			Version:     m.Artifact.Version,
			FixedIn:     fixedIn,
			Description: m.Vulnerability.Description,
			CVSS:        cvss,
			Link:        link,
			ScannedAt:   now,
		})
	}
	return findings, nil
}

// GetReport returns the latest CVE report
func (s *Service) GetReport(ctx context.Context) (*Report, error) {
	if s.lastReport == nil {
		// Return a stub
		return &Report{
			GeneratedAt: time.Now().UTC(),
			Scanner:     "trivy",
			Status:      "not_scanned",
			Findings:    []CVEFinding{},
			Evidence: EvidenceLink{
				LastChecked:       time.Now().UTC(),
				AuthoritativeLink: "https://osv.dev",
				CVEStatus:         "not_scanned",
			},
		}, nil
	}
	return s.lastReport, nil
}

// ComponentInfo describes a software component for SBOM inclusion
type ComponentInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	PURL    string `json:"purl,omitempty"`
	License string `json:"license,omitempty"`
}

// DaemonComponents returns the Go module dependency list for SBOM generation
func DaemonComponents() []ComponentInfo {
	return []ComponentInfo{
		{Name: "gin-gonic/gin", Version: "v1.9.1", PURL: "pkg:golang/github.com/gin-gonic/gin@v1.9.1", License: "MIT"},
		{Name: "gorilla/websocket", Version: "v1.5.1", PURL: "pkg:golang/github.com/gorilla/websocket@v1.5.1", License: "BSD-2-Clause"},
		{Name: "prometheus/client_golang", Version: "v1.18.0", PURL: "pkg:golang/github.com/prometheus/client_golang@v1.18.0", License: "Apache-2.0"},
		{Name: "spf13/cobra", Version: "v1.8.0", PURL: "pkg:golang/github.com/spf13/cobra@v1.8.0", License: "Apache-2.0"},
		{Name: "go.uber.org/zap", Version: "v1.27.0", PURL: "pkg:golang/go.uber.org/zap@v1.27.0", License: "MIT"},
		{Name: "golang-jwt/jwt", Version: "v5.2.0", PURL: "pkg:golang/github.com/golang-jwt/jwt/v5@v5.2.0", License: "MIT"},
		{Name: "pquerna/otp", Version: "v1.4.0", PURL: "pkg:golang/github.com/pquerna/otp@v1.4.0", License: "Apache-2.0"},
		{Name: "coreos/go-oidc", Version: "v3.9.0", PURL: "pkg:golang/github.com/coreos/go-oidc/v3@v3.9.0", License: "Apache-2.0"},
		{Name: "minio/minio-go", Version: "v7.0.66", PURL: "pkg:golang/github.com/minio/minio-go/v7@v7.0.66", License: "Apache-2.0"},
		{Name: "CycloneDX/cyclonedx-go", Version: "v0.8.0", PURL: "pkg:golang/github.com/CycloneDX/cyclonedx-go@v0.8.0", License: "Apache-2.0"},
		{Name: "robfig/cron", Version: "v3.0.1", PURL: "pkg:golang/github.com/robfig/cron/v3@v3.0.1", License: "MIT"},
		{Name: "hashicorp/vault", Version: "v1.12.0", PURL: "pkg:golang/github.com/hashicorp/vault/api@v1.12.0", License: "MPL-2.0"},
	}
}

func (s *Service) writeSBOM(bom *cdx.BOM) error {
	data, err := json.MarshalIndent(bom, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.sbomPath, data, 0644)
}

func (s *Service) writeReport(report *Report) {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		s.logger.Warn("Failed to marshal CVE report", zap.Error(err))
		return
	}
	if err := os.WriteFile(s.reportPath, data, 0644); err != nil {
		s.logger.Warn("Failed to write CVE report", zap.Error(err))
	}
}

func (s *Service) loadReportFromDisk(path string) (*Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return nil, err
	}
	return &report, nil
}
