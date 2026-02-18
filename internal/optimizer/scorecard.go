package optimizer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/mhingston/codefoundry/internal/artifact"
	"github.com/mhingston/codefoundry/internal/flake"
	"github.com/mhingston/codefoundry/internal/gate"
	"github.com/mhingston/codefoundry/internal/protocol"
	"github.com/mhingston/codefoundry/internal/replay"
	"github.com/mhingston/codefoundry/internal/review"
)

const (
	SchemaVersion         = "codefoundry_optimizer_score.v1"
	DefaultSchemaFileName = "optimizer-score.schema.json"
)

type ScoreInput struct {
	RunID             string        `json:"run_id"`
	Timestamp         time.Time     `json:"timestamp"`
	GatePassRate      float64       `json:"gate_pass_rate"`
	ReviewConfidence  float64       `json:"review_confidence"`
	P1Count           int           `json:"p1_count"`
	P2Count           int           `json:"p2_count"`
	P3Count           int           `json:"p3_count"`
	ReplayDeterminism float64       `json:"replay_determinism"`
	CycleTime         time.Duration `json:"cycle_time"`
}

type DimensionScore struct {
	Name         string  `json:"name"`
	Weight       float64 `json:"weight"`
	RawValue     float64 `json:"raw_value"`
	Contribution float64 `json:"contribution"`
}

type Scorecard struct {
	SchemaVersion string           `json:"schema_version"`
	RunID         string           `json:"run_id"`
	Timestamp     time.Time        `json:"timestamp"`
	TotalScore    float64          `json:"total_score"`
	Dimensions    []DimensionScore `json:"dimensions"`
	Inputs        ScoreInput       `json:"inputs"`
}

type Suggestion struct {
	Dimension string `json:"dimension"`
	Reason    string `json:"reason"`
	Action    string `json:"action"`
}

func ComputeScore(input ScoreInput) *Scorecard {
	if input.Timestamp.IsZero() {
		input.Timestamp = time.Now().UTC()
	}

	severityScore := clamp(1.0-(float64(input.P1Count)*0.50+float64(input.P2Count)*0.20+float64(input.P3Count)*0.05), 0, 1)
	cycleScore := 1.0
	if input.CycleTime > 0 {
		target := 2 * time.Hour
		if input.CycleTime > target {
			cycleScore = clamp(float64(target)/float64(input.CycleTime), 0, 1)
		}
	}

	dimensions := []DimensionScore{
		buildDimension("gate_pass_rate", 0.25, input.GatePassRate),
		buildDimension("review_confidence", 0.20, input.ReviewConfidence),
		buildDimension("severity_health", 0.25, severityScore),
		buildDimension("replay_determinism", 0.20, input.ReplayDeterminism),
		buildDimension("cycle_time", 0.10, cycleScore),
	}

	total := 0.0
	for _, dim := range dimensions {
		total += dim.Contribution
	}

	return &Scorecard{
		SchemaVersion: SchemaVersion,
		RunID:         input.RunID,
		Timestamp:     input.Timestamp,
		TotalScore:    math.Round(total*100) / 100,
		Dimensions:    dimensions,
		Inputs:        input,
	}
}

func buildDimension(name string, weight, raw float64) DimensionScore {
	raw = clamp(raw, 0, 1)
	contribution := raw * weight * 100
	return DimensionScore{Name: name, Weight: weight, RawValue: raw, Contribution: math.Round(contribution*100) / 100}
}

func SuggestTweaks(score *Scorecard, limit int) []Suggestion {
	if score == nil {
		return nil
	}
	if limit <= 0 {
		limit = 3
	}

	dims := make([]DimensionScore, len(score.Dimensions))
	copy(dims, score.Dimensions)
	sort.Slice(dims, func(i, j int) bool { return dims[i].RawValue < dims[j].RawValue })

	suggestions := make([]Suggestion, 0, limit)
	for _, dim := range dims {
		if len(suggestions) >= limit {
			break
		}
		suggestions = append(suggestions, suggestionForDimension(dim))
	}
	return suggestions
}

func suggestionForDimension(dim DimensionScore) Suggestion {
	switch dim.Name {
	case "gate_pass_rate":
		return Suggestion{Dimension: dim.Name, Reason: "Gate pass rate is suppressing optimizer score", Action: "Tighten protocol gate ordering and add pre-gate sanity hooks to catch failures earlier."}
	case "review_confidence":
		return Suggestion{Dimension: dim.Name, Reason: "Low review confidence indicates uncertain implementation quality", Action: "Increase harness prompt specificity and require explicit acceptance criteria in stage templates."}
	case "severity_health":
		return Suggestion{Dimension: dim.Name, Reason: "Severity-weighted findings are dragging score down", Action: "Prioritize P1/P2 remediation and add protocol-level blocking checks for recurring severity classes."}
	case "replay_determinism":
		return Suggestion{Dimension: dim.Name, Reason: "Replay determinism is low, indicating non-deterministic workflow behavior", Action: "Stabilize harness inputs (fixed seeds, pinned tool versions) and add deterministic replay fixtures."}
	case "cycle_time":
		return Suggestion{Dimension: dim.Name, Reason: "Cycle time is above target and reducing score", Action: "Split slow stages, parallelize independent gates, and reduce expensive harness retries."}
	default:
		return Suggestion{Dimension: dim.Name, Reason: "Dimension is underperforming", Action: "Tune protocol and harness settings to improve this metric."}
	}
}

func SaveScorecard(store *artifact.Store, score *Scorecard) error {
	if score == nil {
		return fmt.Errorf("scorecard is required")
	}

	schemaPath := filepath.Join("schemas", DefaultSchemaFileName)
	if _, err := os.Stat(schemaPath); err == nil {
		if err := ValidateScorecard(score, schemaPath); err != nil {
			return err
		}
	}

	data, err := json.MarshalIndent(score, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal scorecard: %w", err)
	}
	if err := store.Write("optimizer", "score.json", data); err != nil {
		return fmt.Errorf("failed to write optimizer scorecard: %w", err)
	}
	return nil
}

func LoadScorecard(store *artifact.Store) (*Scorecard, error) {
	data, err := store.Read("optimizer", "score.json")
	if err != nil {
		return nil, err
	}
	var score Scorecard
	if err := json.Unmarshal(data, &score); err != nil {
		return nil, fmt.Errorf("failed to unmarshal optimizer scorecard: %w", err)
	}
	return &score, nil
}

func ValidateScorecard(score *Scorecard, schemaPath string) error {
	validator := protocol.NewValidator()
	if err := validator.Validate(schemaPath, score); err != nil {
		return fmt.Errorf("optimizer scorecard schema validation failed: %w", err)
	}
	return nil
}

func BuildInputFromArtifacts(runID string, store *artifact.Store) (ScoreInput, error) {
	input := ScoreInput{RunID: runID}

	if traceData, err := store.Read("_trace", "execution-trace.json"); err == nil {
		var trace struct {
			Timestamp  time.Time `json:"timestamp"`
			DurationMs int64     `json:"duration_ms"`
		}
		if json.Unmarshal(traceData, &trace) == nil {
			input.Timestamp = trace.Timestamp
			input.CycleTime = time.Duration(trace.DurationMs) * time.Millisecond
		}
	}

	if reviewData, err := store.Read("review", "review-result.json"); err == nil {
		if rr, err := review.FromJSON(reviewData); err == nil {
			input.ReviewConfidence = rr.ConfidenceScore
			input.P1Count = rr.P1Count
			input.P2Count = rr.P2Count
			input.P3Count = rr.P3Count
		}
	}

	gateResults := loadGateResults(store)
	if len(gateResults) > 0 {
		passes := 0
		for _, result := range gateResults {
			if strings.EqualFold(result.Status, "pass") {
				passes++
			}
		}
		input.GatePassRate = float64(passes) / float64(len(gateResults))
	}

	if flakeReport, err := flake.LoadReport(store); err == nil {
		input.ReplayDeterminism = flakeReport.SuccessRate
	} else if replayResult, err := replay.LoadReplayResult(store); err == nil {
		if replayResult.Matches {
			input.ReplayDeterminism = 1
		}
	}

	return input, nil
}

func ComputeFromArtifacts(runID string, store *artifact.Store) (*Scorecard, error) {
	input, err := BuildInputFromArtifacts(runID, store)
	if err != nil {
		return nil, err
	}
	return ComputeScore(input), nil
}

func loadGateResults(store *artifact.Store) []gate.GateResult {
	results := make([]gate.GateResult, 0)
	entries, err := os.ReadDir(store.Namespace().RunPath())
	if err != nil {
		return results
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), "_") {
			continue
		}
		artifacts, err := store.List(entry.Name())
		if err != nil {
			continue
		}
		for _, name := range artifacts {
			if !strings.HasSuffix(name, ".json") {
				continue
			}
			data, err := store.Read(entry.Name(), name)
			if err != nil {
				continue
			}
			var result gate.GateResult
			if err := json.Unmarshal(data, &result); err == nil && result.SchemaVersion == "codefoundry_gate_report.v1" {
				results = append(results, result)
			}
		}
	}
	return results
}

func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
