package report

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"testing"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
)

func setupBundleTest(t *testing.T) (*artifact.Store, string, func()) {
	tmpDir, err := os.MkdirTemp("", "bundle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}

	ns := artifact.NewNamespace(tmpDir, "test-run")
	store := artifact.NewStore(ns)

	cleanup := func() {
		os.RemoveAll(tmpDir)
	}

	return store, tmpDir, cleanup
}

func createBundleArtifacts(store *artifact.Store, basePath, runID string) error {
	stageID := "test-stage"

	// Create gate reports
	gate1 := gate.GateResult{
		SchemaVersion: "codefoundry_gate_report.v1",
		GateID:        "lint",
		Status:        "pass",
	}
	if err := store.WriteJSON(stageID, "lint.json", gate1); err != nil {
		return err
	}

	gate2 := gate.GateResult{
		SchemaVersion: "codefoundry_gate_report.v1",
		GateID:        "test",
		Status:        "pass",
	}
	if err := store.WriteJSON(stageID, "test.json", gate2); err != nil {
		return err
	}

	// Create status
	status := map[string]interface{}{
		"schema_version": "codefoundry_stage_status.v1",
		"stage_id":       stageID,
		"status":         "pass",
	}
	if err := store.WriteJSON(stageID, "status.json", status); err != nil {
		return err
	}

	return nil
}

func TestNewBundleCreator(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	creator := NewBundleCreator(store, tmpDir)

	if creator == nil {
		t.Fatal("NewBundleCreator returned nil")
	}

	if creator.artifactStore != store {
		t.Error("artifactStore not set correctly")
	}
}

func TestBundleCreator_CreateBundle(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	outputPath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", outputPath); err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Verify bundle was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("CreateBundle() did not create output file")
	}

	// Verify it's a valid bundle
	if err := VerifyBundle(outputPath); err != nil {
		t.Errorf("VerifyBundle() error = %v", err)
	}
}

func TestBundleCreator_CreateBundle_DefaultPath(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Change to tmpDir so bundle is created there
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	creator := NewBundleCreator(store, tmpDir)

	// Don't specify output path
	if err := creator.CreateBundle("test-run", ""); err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Should create codefoundry-evidence-test-run.tar.gz
	expectedPath := "codefoundry-evidence-test-run.tar.gz"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("CreateBundle() did not create default output file: %s", expectedPath)
	}
}

func TestBundleCreator_CreateBundle_WithMetadata(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	outputPath := filepath.Join(tmpDir, "bundle.tar.gz")

	metadata := map[string]interface{}{
		"version": "1.0.0",
		"author":  "test",
	}

	if err := creator.CreateBundleWithMetadata("test-run", outputPath, metadata); err != nil {
		t.Errorf("CreateBundleWithMetadata() error = %v", err)
	}

	// Verify bundle was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("CreateBundleWithMetadata() did not create output file")
	}
}

func TestExtractBundle(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")
	extractDir := filepath.Join(tmpDir, "extracted")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Fatalf("Failed to create bundle: %v", err)
	}

	if err := ExtractBundle(bundlePath, extractDir); err != nil {
		t.Errorf("ExtractBundle() error = %v", err)
	}

	// Verify extraction
	bundleInfoPath := filepath.Join(extractDir, "evidence", "bundle-info.json")
	if _, err := os.Stat(bundleInfoPath); os.IsNotExist(err) {
		t.Errorf("ExtractBundle() did not extract bundle-info.json")
	}
}

func TestVerifyBundle(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Fatalf("Failed to create bundle: %v", err)
	}

	// Should pass
	if err := VerifyBundle(bundlePath); err != nil {
		t.Errorf("VerifyBundle() error = %v", err)
	}

	// Test with invalid bundle
	invalidPath := filepath.Join(tmpDir, "invalid.tar.gz")
	if err := os.WriteFile(invalidPath, []byte("not a valid bundle"), 0644); err != nil {
		t.Fatalf("Failed to write invalid bundle: %v", err)
	}

	if err := VerifyBundle(invalidPath); err == nil {
		t.Error("VerifyBundle() should fail for invalid bundle")
	}
}

func TestGetBundleInfo(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Fatalf("Failed to create bundle: %v", err)
	}

	info, err := GetBundleInfo(bundlePath)
	if err != nil {
		t.Errorf("GetBundleInfo() error = %v", err)
	}

	if info == nil {
		t.Fatal("GetBundleInfo() returned nil")
	}

	if info["run_id"] != "test-run" {
		t.Errorf("run_id = %v, want test-run", info["run_id"])
	}

	if info["schema_version"] != "codefoundry_evidence_bundle.v1" {
		t.Errorf("schema_version = %v, want codefoundry_evidence_bundle.v1", info["schema_version"])
	}
}

func TestBundleCreator_addStageArtifacts(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Extract and verify structure
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := ExtractBundle(bundlePath, extractDir); err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	// Check that gate reports are in the right place
	lintPath := filepath.Join(extractDir, "evidence", "gate-reports", "lint.json")
	if _, err := os.Stat(lintPath); os.IsNotExist(err) {
		t.Error("Gate report not in gate-reports directory")
	}

	// Check status is renamed
	statusPath := filepath.Join(extractDir, "evidence", "status-test-stage.json")
	if _, err := os.Stat(statusPath); os.IsNotExist(err) {
		t.Error("Status file not renamed correctly")
	}
}

func TestBundleCreator_isGateReport(t *testing.T) {
	creator := &BundleCreator{}

	gateData := []byte(`{"schema_version": "codefoundry_gate_report.v1", "gate_id": "test"}`)
	if !creator.isGateReport(gateData) {
		t.Error("isGateReport() should return true for gate report")
	}

	otherData := []byte(`{"schema_version": "other", "gate_id": "test"}`)
	if creator.isGateReport(otherData) {
		t.Error("isGateReport() should return false for non-gate report")
	}

	invalidData := []byte(`not json`)
	if creator.isGateReport(invalidData) {
		t.Error("isGateReport() should return false for invalid JSON")
	}
}

func TestBundleCreator_CreateBundle_NoArtifactStore(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bundle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a minimal artifact structure manually
	artifactDir := filepath.Join(tmpDir, "artifacts", "test-run", "test-stage")
	if err := os.MkdirAll(artifactDir, 0755); err != nil {
		t.Fatalf("Failed to create artifact dir: %v", err)
	}

	// Create a status file
	statusContent := `{"schema_version": "codefoundry_stage_status.v1", "stage_id": "test-stage", "status": "pass"}`
	if err := os.WriteFile(filepath.Join(artifactDir, "status.json"), []byte(statusContent), 0644); err != nil {
		t.Fatalf("Failed to write status: %v", err)
	}

	// Create bundle without artifact store
	creator := NewBundleCreator(nil, tmpDir)
	outputPath := filepath.Join(tmpDir, "bundle.tar.gz")

	err = creator.CreateBundle("test-run", outputPath)
	if err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Verify bundle was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("CreateBundle() did not create output file")
	}
}

func TestBundleCreator_CreateBundle_DefaultPathExtension(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Change to tmpDir so bundle is created there
	originalDir, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(originalDir)

	creator := NewBundleCreator(store, tmpDir)

	// Specify path without extension
	if err := creator.CreateBundle("test-run", "my-bundle"); err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Should create my-bundle.tar.gz
	expectedPath := "my-bundle.tar.gz"
	if _, err := os.Stat(expectedPath); os.IsNotExist(err) {
		t.Errorf("CreateBundle() did not create file with extension: %s", expectedPath)
	}
}

func TestBundleCreator_CreateBundle_WithReviewAndLock(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Add review result
	reviewResult := `{
		"schema_version": "codefoundry_review_result.v1",
		"rubric_score": 80,
		"confidence_score": 0.85,
		"dimensions": {
			"correctness": 4,
			"efficiency": 4,
			"maintainability": 4,
			"safety": 4
		},
		"findings": [],
		"summary": "Test"
	}`
	store.Write("test-stage", "review-result.json", []byte(reviewResult))

	// Add lock decision
	lockDecision := `{
		"schema_version": "codefoundry_lock_decision.v1",
		"decision": "resolved",
		"reason": "All checks passed",
		"timestamp": "2024-01-15T10:00:00Z"
	}`
	store.Write("test-stage", "lock-decision.json", []byte(lockDecision))

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Extract and verify structure
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := ExtractBundle(bundlePath, extractDir); err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	// Check review file is in review directory
	reviewPath := filepath.Join(extractDir, "evidence", "review", "review-report.json")
	if _, err := os.Stat(reviewPath); os.IsNotExist(err) {
		t.Error("Review report not in review directory")
	}

	// Check lock decision is in lock directory
	lockPath := filepath.Join(extractDir, "evidence", "lock", "lock-decision.json")
	if _, err := os.Stat(lockPath); os.IsNotExist(err) {
		t.Error("Lock decision not in lock directory")
	}
}

func TestBundleCreator_CreateBundle_WithAdditionalFiles(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	// Add additional file types
	store.Write("test-stage", "log.txt", []byte("Test log content"))
	store.Write("test-stage", "doc.md", []byte("# Documentation"))
	store.Write("test-stage", "other.bin", []byte("binary data"))

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Extract and verify
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := ExtractBundle(bundlePath, extractDir); err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	// Check files are in appropriate directories
	logPath := filepath.Join(extractDir, "evidence", "logs", "log.txt")
	docPath := filepath.Join(extractDir, "evidence", "docs", "doc.md")

	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Error("Log file not in logs directory")
	}

	if _, err := os.Stat(docPath); os.IsNotExist(err) {
		t.Error("Doc file not in docs directory")
	}
}

func TestExtractBundle_InvalidBundle(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bundle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create invalid bundle file
	invalidPath := filepath.Join(tmpDir, "invalid.tar.gz")
	if err := os.WriteFile(invalidPath, []byte("not a valid bundle"), 0644); err != nil {
		t.Fatalf("Failed to write invalid bundle: %v", err)
	}

	extractDir := filepath.Join(tmpDir, "extracted")
	err = ExtractBundle(invalidPath, extractDir)
	if err == nil {
		t.Error("ExtractBundle() should fail for invalid bundle")
	}
}

func TestExtractBundle_NonExistentFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bundle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	extractDir := filepath.Join(tmpDir, "extracted")
	err = ExtractBundle("/nonexistent/bundle.tar.gz", extractDir)
	if err == nil {
		t.Error("ExtractBundle() should fail for non-existent file")
	}
}

func TestVerifyBundle_MissingBundleInfo(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Fatalf("Failed to create bundle: %v", err)
	}

	// Should pass verification
	if err := VerifyBundle(bundlePath); err != nil {
		t.Errorf("VerifyBundle() error = %v", err)
	}
}

func TestVerifyBundle_CorruptedGzip(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bundle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a corrupted gzip file (valid gzip header but corrupted data)
	corruptedPath := filepath.Join(tmpDir, "corrupted.tar.gz")
	corruptedData := []byte{0x1f, 0x8b, 0x08, 0x00} // Partial gzip header
	if err := os.WriteFile(corruptedPath, corruptedData, 0644); err != nil {
		t.Fatalf("Failed to write corrupted bundle: %v", err)
	}

	if err := VerifyBundle(corruptedPath); err == nil {
		t.Error("VerifyBundle() should fail for corrupted gzip")
	}
}

func TestGetBundleInfo_NotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bundle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a tar.gz without bundle-info.json
	bundlePath := filepath.Join(tmpDir, "no-info.tar.gz")
	
	// Create minimal tar.gz
	outFile, _ := os.Create(bundlePath)
	gzWriter := gzip.NewWriter(outFile)
	tw := tar.NewWriter(gzWriter)
	
	// Add a file that's not bundle-info.json
	header := &tar.Header{
		Name: "other-file.txt",
		Mode: 0644,
		Size: 4,
	}
	tw.WriteHeader(header)
	tw.Write([]byte("test"))
	
	tw.Close()
	gzWriter.Close()
	outFile.Close()

	_, err = GetBundleInfo(bundlePath)
	if err == nil {
		t.Error("GetBundleInfo() should fail when bundle-info.json not found")
	}
}

func TestGetBundleInfo_InvalidJSON(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "bundle-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a tar.gz with invalid bundle-info.json
	bundlePath := filepath.Join(tmpDir, "invalid-info.tar.gz")
	
	outFile, _ := os.Create(bundlePath)
	gzWriter := gzip.NewWriter(outFile)
	tw := tar.NewWriter(gzWriter)
	
	header := &tar.Header{
		Name: "evidence/bundle-info.json",
		Mode: 0644,
		Size: 10,
	}
	tw.WriteHeader(header)
	tw.Write([]byte("not valid"))
	
	tw.Close()
	gzWriter.Close()
	outFile.Close()

	_, err = GetBundleInfo(bundlePath)
	if err == nil {
		t.Error("GetBundleInfo() should fail for invalid JSON")
	}
}

func TestBundleCreator_CreateBundleWithMetadata(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	outputPath := filepath.Join(tmpDir, "bundle.tar.gz")

	metadata := map[string]interface{}{
		"version":     "1.0.0",
		"author":      "test-user",
		"build_id":    "12345",
		"custom_key":  "custom_value",
	}

	if err := creator.CreateBundleWithMetadata("test-run", outputPath, metadata); err != nil {
		t.Errorf("CreateBundleWithMetadata() error = %v", err)
	}

	// Verify bundle was created
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("CreateBundleWithMetadata() did not create output file")
	}

	// Verify it's a valid bundle
	if err := VerifyBundle(outputPath); err != nil {
		t.Errorf("VerifyBundle() error = %v", err)
	}
}

func TestBundleCreator_addJSONToTar(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	if err := createBundleArtifacts(store, tmpDir, "test-run"); err != nil {
		t.Fatalf("Failed to create artifacts: %v", err)
	}

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	outFile, err := os.Create(bundlePath)
	if err != nil {
		t.Fatalf("Failed to create bundle file: %v", err)
	}
	defer outFile.Close()

	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	tw := tar.NewWriter(gzWriter)
	defer tw.Close()

	// Test addJSONToTar with valid data
	data := map[string]interface{}{
		"key": "value",
	}
	err = creator.addJSONToTar(tw, "test.json", data)
	if err != nil {
		t.Errorf("addJSONToTar() error = %v", err)
	}

	// Close writers to flush
	tw.Close()
	gzWriter.Close()

	// Verify bundle was created
	if _, err := os.Stat(bundlePath); os.IsNotExist(err) {
		t.Error("addJSONToTar() did not create file")
	}
}

func TestBundleCreator_determineBundlePath(t *testing.T) {
	creator := &BundleCreator{}

	tests := []struct {
		filename string
		expected string
	}{
		{"report.json", "gate-reports"},
		{"doc.md", "docs"},
		{"log.txt", "logs"},
		{"data.yaml", "artifacts"},
		{"config.yml", "artifacts"},
		{"", "artifacts"},
		{"noext", "artifacts"},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			got := creator.determineBundlePath(tt.filename)
			if got != tt.expected {
				t.Errorf("determineBundlePath(%q) = %q, want %q", tt.filename, got, tt.expected)
			}
		})
	}
}

func TestBundleCreator_MultipleStages(t *testing.T) {
	store, tmpDir, cleanup := setupBundleTest(t)
	defer cleanup()

	// Create multiple stages
	stageIDs := []string{"stage-1", "stage-2", "stage-3"}
	for _, stageID := range stageIDs {
		// Create status
		status := map[string]interface{}{
			"schema_version": "codefoundry_stage_status.v1",
			"stage_id":       stageID,
			"status":         "pass",
		}
		store.WriteJSON(stageID, "status.json", status)

		// Create a gate report
		gateResult := gate.GateResult{
			SchemaVersion: "codefoundry_gate_report.v1",
			GateID:        stageID + "-gate",
			Status:        "pass",
		}
		store.WriteJSON(stageID, stageID+"-gate.json", gateResult)
	}

	creator := NewBundleCreator(store, tmpDir)
	bundlePath := filepath.Join(tmpDir, "bundle.tar.gz")

	if err := creator.CreateBundle("test-run", bundlePath); err != nil {
		t.Errorf("CreateBundle() error = %v", err)
	}

	// Extract and verify
	extractDir := filepath.Join(tmpDir, "extracted")
	if err := ExtractBundle(bundlePath, extractDir); err != nil {
		t.Fatalf("Failed to extract bundle: %v", err)
	}

	// Check all stages are present
	for _, stageID := range stageIDs {
		statusPath := filepath.Join(extractDir, "evidence", "status-"+stageID+".json")
		if _, err := os.Stat(statusPath); os.IsNotExist(err) {
			t.Errorf("Status file not found for stage %s", stageID)
		}
	}
}
