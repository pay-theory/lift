package security

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingRiskModel struct {
	updateCalls int
	updates     []RiskFeedback
}

func (r *recordingRiskModel) Predict([]float64) (float64, error)            { return 0, nil }
func (r *recordingRiskModel) Train([]TrainingExample) error                { return nil }
func (r *recordingRiskModel) GetFeatureImportance() map[string]float64      { return map[string]float64{} }
func (r *recordingRiskModel) GetModelMetrics() *ModelMetrics                { return &ModelMetrics{} }
func (r *recordingRiskModel) Update(feedback []RiskFeedback) error {
	r.updateCalls++
	r.updates = append(r.updates, feedback...)
	return nil
}

func TestMLRiskScorer_UpdateRiskModelAndGetRiskFactors(t *testing.T) {
	t.Parallel()

	scorer := NewMLRiskScorer(RiskScoringConfig{Enabled: true, AdaptiveLearning: false})
	require.Error(t, scorer.UpdateRiskModel(context.Background(), []*RiskFeedback{{EventID: "e1"}}))

	scorer = NewMLRiskScorer(RiskScoringConfig{Enabled: true, AdaptiveLearning: true})
	require.NoError(t, scorer.UpdateRiskModel(context.Background(), []*RiskFeedback{nil, {EventID: "e1"}}))
	require.Len(t, scorer.feedback, 1)

	model := &recordingRiskModel{}
	scorer.SetModel(model)
	require.NoError(t, scorer.UpdateRiskModel(context.Background(), []*RiskFeedback{{EventID: "e2"}, nil}))
	assert.Equal(t, 1, model.updateCalls)
	assert.Len(t, model.updates, 1)
	assert.Equal(t, "e2", model.updates[0].EventID)

	factors := scorer.GetRiskFactors()
	require.NotEmpty(t, factors)
	originalID := factors[0].ID
	factors[0].ID = "changed"
	factors2 := scorer.GetRiskFactors()
	assert.Equal(t, originalID, factors2[0].ID, "GetRiskFactors returns a copy")
}

func TestMLRiskScorer_HelperCoverage(t *testing.T) {
	t.Parallel()

	scorer := newEnabledRiskScorer()

	assert.Equal(t, 0.8, scorer.getDurationRisk(50*time.Millisecond))
	assert.Equal(t, 0.5, scorer.getDurationRisk(500*time.Millisecond))
	assert.Equal(t, 0.2, scorer.getDurationRisk(10*time.Second))
	assert.Equal(t, 0.8, scorer.getDurationRisk(400*time.Second))

	// Recommendations include both risk-level and factor mitigation messages.
	factors := []RiskFactor{{Mitigation: "do something"}}
	assert.Contains(t, scorer.generateRecommendations(riskLevelCritical, factors), "Immediate investigation required")
	assert.Contains(t, scorer.generateRecommendations(riskLevelHigh, factors), "Enhanced monitoring required")
	assert.Contains(t, scorer.generateRecommendations(riskLevelMedium, factors), "Monitor for patterns")
	assert.Contains(t, scorer.generateRecommendations(riskLevelLow, factors), "Continue normal monitoring")

	assert.Equal(t, "stable", scorer.calculateTrendDirection([]*AuditEvent{{Timestamp: time.Now()}}))

	increasing := []*AuditEvent{
		{Timestamp: time.Unix(1, 0), Severity: riskLevelLow},
		{Timestamp: time.Unix(2, 0), Severity: riskLevelLow},
		{Timestamp: time.Unix(3, 0), Severity: riskLevelCritical},
		{Timestamp: time.Unix(4, 0), Severity: riskLevelCritical},
	}
	assert.Equal(t, "increasing", scorer.calculateTrendDirection(increasing))

	decreasing := []*AuditEvent{
		{Timestamp: time.Unix(1, 0), Severity: riskLevelCritical},
		{Timestamp: time.Unix(2, 0), Severity: riskLevelCritical},
		{Timestamp: time.Unix(3, 0), Severity: riskLevelLow},
		{Timestamp: time.Unix(4, 0), Severity: riskLevelLow},
	}
	assert.Equal(t, "decreasing", scorer.calculateTrendDirection(decreasing))

	stable := []*AuditEvent{
		{Timestamp: time.Unix(1, 0), Severity: riskLevelMedium},
		{Timestamp: time.Unix(2, 0), Severity: riskLevelMedium},
		{Timestamp: time.Unix(3, 0), Severity: riskLevelMedium},
		{Timestamp: time.Unix(4, 0), Severity: riskLevelMedium},
	}
	assert.Equal(t, "stable", scorer.calculateTrendDirection(stable))

	tr := scorer.getTimeRange([]*AuditEvent{
		{Timestamp: time.Unix(100, 0)},
		{Timestamp: time.Unix(10, 0)},
		{Timestamp: time.Unix(50, 0)},
	})
	assert.Equal(t, time.Unix(10, 0), tr.Start)
	assert.Equal(t, time.Unix(100, 0), tr.End)

	empty := scorer.getTimeRange(nil)
	assert.True(t, empty.Start.Equal(empty.End))
}

func TestMLRiskScorer_DefaultMappings(t *testing.T) {
	t.Parallel()

	scorer := newEnabledRiskScorer()

	assert.Equal(t, 0.5, scorer.getEventTypeRisk("unknown"))
	assert.Equal(t, 0.5, scorer.getActionRisk("unknown"))
	assert.Equal(t, 0.3, scorer.getSeverityRisk("unknown"))
	assert.Equal(t, 0.5, scorer.getDataTypeRisk("unknown"))

	assert.Equal(t, 0.8, scorer.getComplianceRisk(ComplianceContext{RiskLevel: riskLevelCritical}))
	assert.Equal(t, 0.3, scorer.getComplianceRisk(ComplianceContext{}))

	assert.Equal(t, 0.0, scorer.getSecurityContextRisk(SecurityContext{ThreatLevel: riskLevelLow, EncryptionUsed: true}))
	assert.Greater(t, scorer.getSecurityContextRisk(SecurityContext{ThreatLevel: riskLevelHigh, EncryptionUsed: false, ThreatIndicators: []string{"x"}}), 0.0)
}
