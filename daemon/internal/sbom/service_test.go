package sbom

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"go.uber.org/zap"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	return NewService("", "", zap.NewNop())
}

func TestNewService(t *testing.T) {
	svc := newTestService(t)
	if svc == nil {
		t.Fatal("NewService returned nil")
	}
}

func TestGetSBOM_NoSBOM_ReturnsPlaceholder(t *testing.T) {
	svc := newTestService(t)
	result, err := svc.GetSBOM(context.Background())
	if err != nil {
		t.Fatalf("GetSBOM error: %v", err)
	}
	if result == nil {
		t.Fatal("expected placeholder map, got nil")
	}
	if result["bomFormat"] != "CycloneDX" {
		t.Errorf("bomFormat = %v, want CycloneDX", result["bomFormat"])
	}
}

func TestGetReport_NoReport_ReturnsStub(t *testing.T) {
	svc := newTestService(t)
	report, err := svc.GetReport(context.Background())
	if err != nil {
		t.Fatalf("GetReport error: %v", err)
	}
	if report == nil {
		t.Fatal("expected stub report, got nil")
	}
	if report.Status != "not_scanned" {
		t.Errorf("Status = %q, want not_scanned", report.Status)
	}
	if report.Findings == nil {
		t.Error("Findings should not be nil")
	}
}

func TestGenerateSBOM_WithComponents(t *testing.T) {
	svc := newTestService(t)
	components := []ComponentInfo{
		{Name: "gin", Version: "v1.9.1", PURL: "pkg:golang/gin@v1.9.1", License: "MIT"},
		{Name: "zap",  Version: "v1.27.0", PURL: "pkg:golang/zap@v1.27.0"},
	}
	bom, err := svc.GenerateSBOM(context.Background(), components)
	if err != nil {
		t.Fatalf("GenerateSBOM error: %v", err)
	}
	if bom == nil {
		t.Fatal("expected BOM, got nil")
	}
	if bom.Components == nil || len(*bom.Components) != 2 {
		t.Errorf("expected 2 components in BOM, got %v", bom.Components)
	}
}

func TestGenerateSBOM_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	sbomPath := filepath.Join(dir, "sbom.json")
	svc := NewService("", sbomPath, zap.NewNop())

	_, err := svc.GenerateSBOM(context.Background(), []ComponentInfo{
		{Name: "cobra", Version: "v1.8.0"},
	})
	if err != nil {
		t.Fatalf("GenerateSBOM error: %v", err)
	}
	if _, err := os.Stat(sbomPath); os.IsNotExist(err) {
		t.Error("expected SBOM file to be written to disk")
	}
}

func TestGetSBOM_AfterGenerate(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.GenerateSBOM(context.Background(), []ComponentInfo{
		{Name: "cobra", Version: "v1.8.0"},
	})
	if err != nil {
		t.Fatalf("GenerateSBOM error: %v", err)
	}

	result, err := svc.GetSBOM(context.Background())
	if err != nil {
		t.Fatalf("GetSBOM error: %v", err)
	}
	// After generating, should have actual BOM content (not just placeholder)
	if result == nil {
		t.Fatal("expected SBOM result, got nil")
	}
}

func TestTriggerScan_NoScanner_ReturnsReport(t *testing.T) {
	// In a test environment, trivy/grype won't be present — service should handle gracefully
	svc := newTestService(t)
	report, err := svc.TriggerScan(context.Background())
	if err != nil {
		t.Fatalf("TriggerScan error: %v", err)
	}
	if report == nil {
		t.Fatal("expected report, got nil")
	}
	// Without a scanner, status should be "clean" (empty findings)
	if report.Findings == nil {
		t.Error("Findings should not be nil")
	}
}

func TestTriggerScan_PersistsToDisk(t *testing.T) {
	dir := t.TempDir()
	reportPath := filepath.Join(dir, "cve-report.json")
	svc := NewService(reportPath, "", zap.NewNop())

	_, err := svc.TriggerScan(context.Background())
	if err != nil {
		t.Fatalf("TriggerScan error: %v", err)
	}
	if _, err := os.Stat(reportPath); os.IsNotExist(err) {
		t.Error("expected report file to be written to disk")
	}
}

func TestLoadReportFromDisk_Roundtrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "report.json")

	report := &Report{
		Status:     "clean",
		TotalCount: 0,
		Findings:   []CVEFinding{},
	}
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(path, data, 0644)

	svc := newTestService(t)
	loaded, err := svc.loadReportFromDisk(path)
	if err != nil {
		t.Fatalf("loadReportFromDisk error: %v", err)
	}
	if loaded.Status != "clean" {
		t.Errorf("Status = %q, want clean", loaded.Status)
	}
}

func TestDaemonComponents_NonEmpty(t *testing.T) {
	components := DaemonComponents()
	if len(components) == 0 {
		t.Fatal("DaemonComponents should return non-empty slice")
	}
	for _, c := range components {
		if c.Name == "" {
			t.Error("component has empty name")
		}
		if c.Version == "" {
			t.Errorf("component %q has empty version", c.Name)
		}
	}
}
