package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubRiskModel struct {
	prediction        float64
	err               error
	featureImportance map[string]float64
}

func (s *stubRiskModel) Predict(features []float64) (float64, error) {
	_ = features
	return s.prediction, s.err
}

func (s *stubRiskModel) Train(_ []TrainingExample) error {
	return nil
}

func (s *stubRiskModel) Update(_ []RiskFeedback) error {
	return nil
}

func (s *stubRiskModel) GetFeatureImportance() map[string]float64 {
	if s.featureImportance == nil {
		return map[string]float64{}
	}
	return s.featureImportance
}

func (s *stubRiskModel) GetModelMetrics() *ModelMetrics {
	return &ModelMetrics{}
}

func newEnabledRiskScorer() *MLRiskScorer {
	return NewMLRiskScorer(RiskScoringConfig{
		Enabled: true,
		ThresholdConfig: RiskThresholdConfig{
			CriticalThreshold: 80,
			HighThreshold:     60,
			MediumThreshold:   40,
			LowThreshold:      20,
		},
	})
}

func TestMLRiskScorer_CalculateRiskScore_WeightedFallback(t *testing.T) {
	scorer := newEnabledRiskScorer()

	event := &AuditEvent{
		Timestamp:    time.Date(2024, time.April, 6, 23, 0, 0, 0, time.UTC), // Saturday, off-hours
		Severity:     riskLevelCritical,
		EventType:    "security_event",
		Action:       "delete",
		Result:       "failure",
		UserID:       "user-1",
		IPAddress:    "198.51.100.1",
		DataAccessed: []string{"restricted"},
		Compliance: ComplianceContext{
			RiskLevel:  riskLevelHigh,
			Violations: []string{"SOC2"},
		},
		Security: SecurityContext{
			ThreatLevel:      riskLevelHigh,
			EncryptionUsed:   false,
			ThreatIndicators: []string{"indicator-1"},
		},
	}

	score, err := scorer.CalculateRiskScore(context.Background(), event)
	require.NoError(t, err)

	assert.InDelta(t, 86, score.Score, 5, "weighted fallback should yield high score")
	assert.Equal(t, riskLevelCritical, score.Level)
	assert.True(t, score.Metadata["model_used"].(bool) == false)
	assert.Greater(t, len(score.Factors), 0)
	assert.Contains(t, score.Recommendations, "Immediate investigation required")
}

func TestMLRiskScorer_CalculateRiskScore_UsesModelPrediction(t *testing.T) {
	scorer := newEnabledRiskScorer()
	model := &stubRiskModel{prediction: 90}
	scorer.SetModel(model)

	event := &AuditEvent{
		Timestamp: time.Date(2024, time.April, 2, 11, 0, 0, 0, time.UTC),
		Severity:  riskLevelMedium,
		EventType: "authentication",
		Action:    "login",
		Result:    "success",
	}

	score, err := scorer.CalculateRiskScore(context.Background(), event)
	require.NoError(t, err)

	assert.Equal(t, 90.0, score.Score)
	assert.Equal(t, riskLevelCritical, score.Level)
	assert.True(t, score.Metadata["model_used"].(bool))
}

func TestMLRiskScorer_CalculateRiskScore_Disabled(t *testing.T) {
	scorer := NewMLRiskScorer(RiskScoringConfig{Enabled: false})

	_, err := scorer.CalculateRiskScore(context.Background(), &AuditEvent{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestMLRiskScorer_CalculateAggregateRisk(t *testing.T) {
	scorer := newEnabledRiskScorer()

	events := []*AuditEvent{
		{
			Timestamp:    time.Date(2024, time.April, 1, 9, 0, 0, 0, time.UTC),
			Severity:     riskLevelMedium,
			EventType:    "data_access",
			Action:       "read",
			DataAccessed: []string{"confidential"},
		},
		{
			Timestamp:    time.Date(2024, time.April, 2, 22, 0, 0, 0, time.UTC),
			Severity:     riskLevelHigh,
			EventType:    "security_event",
			Action:       "delete",
			DataAccessed: []string{"restricted"},
		},
		{
			Timestamp:    time.Date(2024, time.April, 3, 23, 0, 0, 0, time.UTC),
			Severity:     riskLevelCritical,
			EventType:    "admin_action",
			Action:       "admin",
			DataAccessed: []string{"payment"},
		},
	}

	aggregate, err := scorer.CalculateAggregateRisk(context.Background(), events)
	require.NoError(t, err)

	assert.Equal(t, len(events), aggregate.EventCount)
	assert.Contains(t, aggregate.RiskDistribution, riskLevelCritical)
	assert.Contains(t, []string{"increasing", "stable", "decreasing"}, aggregate.TrendDirection)
	assert.Greater(t, aggregate.OverallScore, 0.0)
	assert.NotEmpty(t, aggregate.TopRiskFactors)

	// Ensure recommendations are deduplicated
	assert.Len(t, aggregate.Recommendations, len(uniqueStrings(aggregate.Recommendations)))
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
