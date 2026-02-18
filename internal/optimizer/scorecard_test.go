package optimizer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestComputeScore(t *testing.T) {
	score := ComputeScore(ScoreInput{
		RunID:             "run-1",
		Timestamp:         time.Now().UTC(),
		GatePassRate:      0.8,
		ReviewConfidence:  0.9,
		P1Count:           0,
		P2Count:           1,
		P3Count:           2,
		ReplayDeterminism: 1,
		CycleTime:         time.Hour,
	})

	assert.Equal(t, SchemaVersion, score.SchemaVersion)
	assert.Equal(t, "run-1", score.RunID)
	assert.Len(t, score.Dimensions, 5)
	assert.Greater(t, score.TotalScore, 0.0)
	assert.LessOrEqual(t, score.TotalScore, 100.0)
}

func TestSuggestTweaks(t *testing.T) {
	score := ComputeScore(ScoreInput{
		RunID:             "run-1",
		Timestamp:         time.Now().UTC(),
		GatePassRate:      0.2,
		ReviewConfidence:  0.3,
		P1Count:           1,
		P2Count:           1,
		P3Count:           0,
		ReplayDeterminism: 0.5,
		CycleTime:         6 * time.Hour,
	})

	suggestions := SuggestTweaks(score, 2)
	assert.Len(t, suggestions, 2)
	assert.NotEmpty(t, suggestions[0].Action)
	assert.NotEmpty(t, suggestions[0].Dimension)
}
