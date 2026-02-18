package report

import (
	"archive/tar"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
)

// BundleCreator creates evidence bundles
type BundleCreator struct {
	artifactStore *artifact.Store
	basePath      string
}

// NewBundleCreator creates a new bundle creator
func NewBundleCreator(artifactStore *artifact.Store, basePath string) *BundleCreator {
	return &BundleCreator{
		artifactStore: artifactStore,
		basePath:      basePath,
	}
}

// CreateBundle creates an evidence bundle for a run
func (b *BundleCreator) CreateBundle(runID string, outputPath string) error {
	// Determine output path
	if outputPath == "" {
		outputPath = fmt.Sprintf("codefoundry-evidence-%s.tar.gz", runID)
	}

	// Ensure .tar.gz extension
	if !strings.HasSuffix(outputPath, ".tar.gz") && !strings.HasSuffix(outputPath, ".tgz") {
		outputPath += ".tar.gz"
	}

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create bundle file: %w", err)
	}
	defer outFile.Close()

	// Create gzip writer
	gzWriter := gzip.NewWriter(outFile)
	defer gzWriter.Close()

	// Create tar writer
	tarWriter := tar.NewWriter(gzWriter)
	defer tarWriter.Close()

	// Add bundle structure
	if err := b.addBundleStructure(tarWriter, runID); err != nil {
		return fmt.Errorf("failed to add bundle structure: %w", err)
	}

	return nil
}

// CreateBundleWithMetadata creates a bundle with additional metadata
func (b *BundleCreator) CreateBundleWithMetadata(
	runID string,
	outputPath string,
	metadata map[string]interface{},
) error {
	// Create metadata file
	metaPath := filepath.Join(b.basePath, "artifacts", runID, "bundle-metadata.json")
	metaData, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaData, 0644); err != nil {
		return fmt.Errorf("failed to write metadata: %w", err)
	}
	defer os.Remove(metaPath)

	return b.CreateBundle(runID, outputPath)
}

// addBundleStructure adds the bundle directory structure
func (b *BundleCreator) addBundleStructure(tw *tar.Writer, runID string) error {
	// Create directories
	dirs := []string{
		"evidence/",
		"evidence/gate-reports/",
		"evidence/review/",
		"evidence/lock/",
	}

	for _, dir := range dirs {
		header := &tar.Header{
			Name:     dir,
			Mode:     0755,
			Typeflag: tar.TypeDir,
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
	}

	// Add bundle info
	bundleInfo := map[string]interface{}{
		"schema_version": "codefoundry_evidence_bundle.v1",
		"run_id":         runID,
		"created_at":     time.Now().UTC().Format(time.RFC3339),
	}

	if err := b.addJSONToTar(tw, "evidence/bundle-info.json", bundleInfo); err != nil {
		return err
	}

	// Discover and add all stage artifacts
	runPath := filepath.Join(b.basePath, "artifacts", runID)
	stages, err := b.discoverStages(runPath)
	if err != nil {
		return fmt.Errorf("failed to discover stages: %w", err)
	}

	for _, stageID := range stages {
		if err := b.addStageArtifacts(tw, runID, stageID); err != nil {
			return fmt.Errorf("failed to add stage %s: %w", stageID, err)
		}
	}

	return nil
}

// addStageArtifacts adds all artifacts from a stage
func (b *BundleCreator) addStageArtifacts(tw *tar.Writer, runID, stageID string) error {
	stagePath := filepath.Join(b.basePath, "artifacts", runID, stageID)

	entries, err := os.ReadDir(stagePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		filePath := filepath.Join(stagePath, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		// Determine target path based on file type
		targetPath := b.determineBundlePath(entry.Name())
		fullPath := fmt.Sprintf("evidence/%s/%s", targetPath, entry.Name())

		// Handle gate reports
		if strings.HasSuffix(entry.Name(), ".json") &&
			entry.Name() != "status.json" &&
			entry.Name() != "review-result.json" &&
			entry.Name() != "lock-decision.json" {
			// Check if it's a gate report
			if b.isGateReport(data) {
				fullPath = fmt.Sprintf("evidence/gate-reports/%s", entry.Name())
			}
		}

		// Handle specific known files
		switch entry.Name() {
		case "review-result.json":
			fullPath = "evidence/review/review-report.json"
		case "lock-decision.json":
			fullPath = "evidence/lock/lock-decision.json"
		case "status.json":
			fullPath = fmt.Sprintf("evidence/status-%s.json", stageID)
		}

		header := &tar.Header{
			Name:    fullPath,
			Mode:    0644,
			Size:    int64(len(data)),
			ModTime: time.Now(),
		}

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		if _, err := tw.Write(data); err != nil {
			return err
		}
	}

	return nil
}

// determineBundlePath determines the appropriate path for a file
func (b *BundleCreator) determineBundlePath(filename string) string {
	switch filepath.Ext(filename) {
	case ".json":
		return "gate-reports"
	case ".md":
		return "docs"
	case ".txt":
		return "logs"
	default:
		return "artifacts"
	}
}

// isGateReport checks if data is a gate report
func (b *BundleCreator) isGateReport(data []byte) bool {
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		return false
	}

	schema, ok := result["schema_version"].(string)
	return ok && schema == "codefoundry_gate_report.v1"
}

// addJSONToTar adds a JSON file to the tar archive
func (b *BundleCreator) addJSONToTar(tw *tar.Writer, path string, data interface{}) error {
	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    path,
		Mode:    0644,
		Size:    int64(len(jsonData)),
		ModTime: time.Now(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	if _, err := tw.Write(jsonData); err != nil {
		return err
	}

	return nil
}

// discoverStages discovers all stages in a run directory
func (b *BundleCreator) discoverStages(runPath string) ([]string, error) {
	entries, err := os.ReadDir(runPath)
	if err != nil {
		return nil, err
	}

	var stages []string
	for _, entry := range entries {
		if entry.IsDir() {
			stages = append(stages, entry.Name())
		}
	}

	return stages, nil
}

// ExtractBundle extracts an evidence bundle
func ExtractBundle(bundlePath, outputDir string) error {
	// Open bundle
	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to open bundle: %w", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %w", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar entry: %w", err)
		}

		targetPath := filepath.Join(outputDir, header.Name)

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}

			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file: %w", err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file: %w", err)
			}
			outFile.Close()
		}
	}

	return nil
}

// VerifyBundle verifies a bundle's integrity
func VerifyBundle(bundlePath string) error {
	// Open bundle
	file, err := os.Open(bundlePath)
	if err != nil {
		return fmt.Errorf("failed to open bundle: %w", err)
	}
	defer file.Close()

	// Check gzip
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to read gzip: %w", err)
	}
	defer gzReader.Close()

	// Check tar
	tarReader := tar.NewReader(gzReader)

	foundBundleInfo := false
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("failed to read tar: %w", err)
		}

		if header.Name == "evidence/bundle-info.json" {
			foundBundleInfo = true

			// Verify JSON
			data := make([]byte, header.Size)
			if _, err := io.ReadFull(tarReader, data); err != nil {
				return fmt.Errorf("failed to read bundle info: %w", err)
			}

			var info map[string]interface{}
			if err := json.Unmarshal(data, &info); err != nil {
				return fmt.Errorf("invalid bundle info JSON: %w", err)
			}

			schema, ok := info["schema_version"].(string)
			if !ok || schema != "codefoundry_evidence_bundle.v1" {
				return fmt.Errorf("invalid bundle schema version")
			}
		}
	}

	if !foundBundleInfo {
		return fmt.Errorf("bundle missing bundle-info.json")
	}

	return nil
}

// GetBundleInfo reads bundle metadata without extracting
func GetBundleInfo(bundlePath string) (map[string]interface{}, error) {
	file, err := os.Open(bundlePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return nil, err
	}
	defer gzReader.Close()

	tarReader := tar.NewReader(gzReader)

	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		if header.Name == "evidence/bundle-info.json" {
			data := make([]byte, header.Size)
			if _, err := io.ReadFull(tarReader, data); err != nil {
				return nil, err
			}

			var info map[string]interface{}
			if err := json.Unmarshal(data, &info); err != nil {
				return nil, err
			}
			return info, nil
		}
	}

	return nil, fmt.Errorf("bundle-info.json not found")
}
