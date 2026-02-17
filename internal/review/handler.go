package review

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/gate"
	"github.com/mhingston/codefoundry/internal/protocol"
)

// Handler executes review stages
type Handler struct {
	artifactStore     *artifact.Store
	classifier        *SeverityClassifier
	templateLoader    *TemplateLoader
	confidenceThreshold float64
}

// HandlerOptions configures the review handler
type HandlerOptions struct {
	ArtifactStore       *artifact.Store
	TemplatePath        string
	ConfidenceThreshold float64
}

// NewHandler creates a new review handler
func NewHandler(opts HandlerOptions) *Handler {
	threshold := opts.ConfidenceThreshold
	if threshold == 0 {
		threshold = 0.7 // Default threshold
	}

	return &Handler{
		artifactStore:       opts.ArtifactStore,
		classifier:          NewSeverityClassifier(),
		templateLoader:      NewTemplateLoader(opts.TemplatePath),
		confidenceThreshold: threshold,
	}
}

// WithArtifactStore sets the artifact store
func (h *Handler) WithArtifactStore(store *artifact.Store) *Handler {
	h.artifactStore = store
	return h
}

// WithConfidenceThreshold sets the confidence threshold
func (h *Handler) WithConfidenceThreshold(threshold float64) *Handler {
	h.confidenceThreshold = threshold
	return h
}

// ExecuteReview executes a review stage
func (h *Handler) ExecuteReview(
	ctx context.Context,
	stage *protocol.Stage,
	gateReports []gate.GateResult,
	diff string,
) (*ReviewResult, error) {
	// Build gate report context
	reports := make([]GateReport, 0, len(gateReports))
	for _, gr := range gateReports {
		reports = append(reports, GateReport{
			GateID:   gr.GateID,
			Status:   gr.Status,
			Failures: gr.Failures,
		})
	}

	// Build template context
	templateCtx := BuildContext(stage.ID, diff, reports)

	// Render review prompt
	prompt, err := h.templateLoader.Render(templateCtx)
	if err != nil {
		return nil, ExecutionError{
			StageID: stage.ID,
			Message: "failed to render review template",
			Cause:   err,
		}
	}

	// Store the prompt as an artifact for audit
	if h.artifactStore != nil {
		if err := h.artifactStore.WriteString(stage.ID, "review-prompt.md", prompt); err != nil {
			return nil, ExecutionError{
				StageID: stage.ID,
				Message: "failed to store review prompt",
				Cause:   err,
			}
		}
	}

	// In a real implementation, this would call the harness/LLM
	// For now, we return a placeholder that the harness would fill
	result := NewReviewResult()
	result.StageID = stage.ID
	result.ConfidenceThreshold = h.confidenceThreshold

	return result, nil
}

// ParseReviewResult parses a raw review result from the harness
func (h *Handler) ParseReviewResult(data []byte, stageID string) (*ReviewResult, error) {
	result, err := FromJSON(data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse review result: %w", err)
	}

	// Ensure counts are set
	result.SetCounts()

	// Classify any unclassified findings
	for i := range result.Findings {
		if result.Findings[i].Severity == "" {
			result.Findings[i].Severity = h.classifier.Classify(result.Findings[i])
		}
	}

	// Update counts after classification
	result.SetCounts()

	// Validate the result
	if err := result.Validate(); err != nil {
		return nil, fmt.Errorf("review result validation failed: %w", err)
	}

	return result, nil
}

// StoreReviewResult stores the review result as an artifact
func (h *Handler) StoreReviewResult(stageID string, result *ReviewResult) error {
	if h.artifactStore == nil {
		return fmt.Errorf("artifact store not configured")
	}

	data, err := result.ToJSON()
	if err != nil {
		return fmt.Errorf("failed to marshal review result: %w", err)
	}

	if err := h.artifactStore.Write(stageID, "review-result.json", data); err != nil {
		return fmt.Errorf("failed to store review result: %w", err)
	}

	return nil
}

// LoadReviewResult loads a review result from artifacts
func (h *Handler) LoadReviewResult(stageID string) (*ReviewResult, error) {
	if h.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}

	data, err := h.artifactStore.Read(stageID, "review-result.json")
	if err != nil {
		return nil, fmt.Errorf("failed to read review result: %w", err)
	}

	return h.ParseReviewResult(data, stageID)
}

// LoadDiff loads the diff from the implementation stage
func (h *Handler) LoadDiff(implementationStageID string) (string, error) {
	if h.artifactStore == nil {
		return "", fmt.Errorf("artifact store not configured")
	}

	// Try to load diff artifact
	data, err := h.artifactStore.Read(implementationStageID, "diff.txt")
	if err != nil {
		// Try diff.json
		data, err = h.artifactStore.Read(implementationStageID, "diff.json")
		if err != nil {
			return "", fmt.Errorf("diff not found for stage %s: %w", implementationStageID, err)
		}
	}

	return string(data), nil
}

// LoadGateReports loads all gate reports for a stage
func (h *Handler) LoadGateReports(stageID string) ([]gate.GateResult, error) {
	if h.artifactStore == nil {
		return nil, fmt.Errorf("artifact store not configured")
	}

	// List all gate report files
	artifacts, err := h.artifactStore.List(stageID)
	if err != nil {
		return nil, fmt.Errorf("failed to list artifacts: %w", err)
	}

	var reports []gate.GateResult
	for _, artifact := range artifacts {
		// Gate reports end with .json and are not special files
		if filepath.Ext(artifact) == ".json" && 
		   artifact != "status.json" && 
		   artifact != "review-result.json" &&
		   artifact != "review-prompt.md" {
			data, err := h.artifactStore.Read(stageID, artifact)
			if err != nil {
				continue // Skip unreadable files
			}

			var result gate.GateResult
			if err := json.Unmarshal(data, &result); err == nil {
				// Successfully parsed as gate result
				if result.SchemaVersion == "codefoundry_gate_report.v1" {
					reports = append(reports, result)
				}
			}
		}
	}

	return reports, nil
}

// ProcessHarnessOutput processes output from the harness and stores result
func (h *Handler) ProcessHarnessOutput(
	stageID string,
	harnessOutput []byte,
) (*ReviewResult, error) {
	result, err := h.ParseReviewResult(harnessOutput, stageID)
	if err != nil {
		return nil, err
	}

	// Store the result
	if err := h.StoreReviewResult(stageID, result); err != nil {
		return nil, err
	}

	return result, nil
}

// DefaultTemplatePath returns the default template path
func DefaultTemplatePath(basePath string) string {
	return filepath.Join(basePath, "templates", "review.md")
}

// EnsureTemplateExists creates the default template if it doesn't exist
func EnsureTemplateExists(basePath string) error {
	templatePath := DefaultTemplatePath(basePath)

	// Check if template exists
	if _, err := os.Stat(templatePath); err == nil {
		return nil // Template exists
	}

	// Create templates directory
	templateDir := filepath.Dir(templatePath)
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return fmt.Errorf("failed to create templates directory: %w", err)
	}

	// Write default template
	if err := os.WriteFile(templatePath, []byte(DefaultTemplate()), 0644); err != nil {
		return fmt.Errorf("failed to write default template: %w", err)
	}

	return nil
}
