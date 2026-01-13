package security

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateStandardRiskAssessment_MergesRiskFactors(t *testing.T) {
	t.Parallel()

	custom := []RiskFactor{
		{ID: "CUSTOM-1", Name: "Custom", Category: "custom", Weight: 0.5},
	}
	assessment := createStandardRiskAssessment(
		"ID",
		"Name",
		"industry",
		[]string{"scope"},
		custom,
		[]string{"threat"},
		[]string{"asset"},
		[]string{"impact"},
	)

	require.Equal(t, "ID", assessment.ID)
	require.Equal(t, "industry", assessment.Industry)
	assert.Equal(t, "NIST", assessment.Methodology)
	assert.Equal(t, 90*24*time.Hour, assessment.Frequency)

	ids := make([]string, 0, len(assessment.RiskFactors))
	for _, rf := range assessment.RiskFactors {
		ids = append(ids, rf.ID)
	}
	assert.Contains(t, ids, "RF-001")
	assert.Contains(t, ids, "RF-002")
	assert.Contains(t, ids, "CUSTOM-1")
}

func TestCreateStandardControls(t *testing.T) {
	t.Parallel()

	access := createStandardAccessControl(
		"AC-1",
		"Access",
		"desc",
		"FW",
		"cat",
		"e1", "d1", false,
		"e2", "d2",
		"remediate",
	)
	assert.Equal(t, "AC-1", access.ID)
	assert.Equal(t, "FW", access.Framework)
	assert.Equal(t, "cat", access.Category)
	assert.True(t, access.Automated)
	require.Len(t, access.Evidence, 2)
	assert.Equal(t, "e1", access.Evidence[0].Type)
	assert.False(t, access.Evidence[0].Automated)
	assert.Equal(t, "e2", access.Evidence[1].Type)
	assert.True(t, access.Evidence[1].Automated)

	monitor := createStandardMonitoringControl(
		"MC-1",
		"Monitor",
		"desc",
		"FW",
		"e1", "d1",
		"e2", "d2",
		"remediate",
	)
	assert.Equal(t, "MC-1", monitor.ID)
	assert.Equal(t, "monitoring", monitor.Category)
	assert.Equal(t, time.Hour, monitor.Frequency)
	require.Len(t, monitor.Evidence, 2)
}

func TestIndustryComplianceTemplateManager_DefaultTemplates(t *testing.T) {
	t.Parallel()

	manager := NewIndustryComplianceTemplateManager()
	require.NotNil(t, manager)

	industries := manager.GetAvailableIndustries()
	assert.ElementsMatch(t, []string{"banking", "healthcare", "ecommerce", "government"}, industries)

	template, err := manager.GetTemplate("banking")
	require.NoError(t, err)
	require.NotNil(t, template)
	assert.Equal(t, "banking", template.GetIndustry())

	_, err = manager.GetTemplate("missing")
	require.Error(t, err)
}

func TestIndustryComplianceTemplates_ExerciseAll(t *testing.T) {
	t.Parallel()

	banking := NewBankingComplianceTemplate(BankingComplianceConfig{
		PCIDSSLevel:     "1",
		SOXCompliance:   true,
		BSACompliance:   true,
		GLBACompliance:  true,
		FedRAMPRequired: true,
		AMLRequired:     true,
		KYCRequired:     true,
	})
	assert.Equal(t, "banking", banking.GetIndustry())
	assert.Contains(t, banking.GetRegulations(), "PCI-DSS")
	assert.Contains(t, banking.GetRegulations(), "SOX")
	assert.Contains(t, banking.GetRegulations(), "BSA")
	assert.Contains(t, banking.GetRegulations(), "GLBA")
	assert.Contains(t, banking.GetRegulations(), "FedRAMP")
	assert.NotEmpty(t, banking.GetControls())
	assert.Len(t, banking.GetRiskAssessments(), 1)
	assert.NotEmpty(t, banking.GetAudits())
	result, err := banking.ValidateCompliance(nil)
	require.NoError(t, err)
	assert.True(t, result.Compliant)
	report, err := banking.GenerateComplianceReport()
	require.NoError(t, err)
	assert.Equal(t, "banking", report.Industry)
	assert.NotEmpty(t, report.Regulations)

	healthcare := NewHealthcareComplianceTemplate(HealthcareComplianceConfig{
		HIPAARequired:      true,
		HITECHRequired:     true,
		FDACompliance:      true,
		DEACompliance:      true,
		BreachNotification: true,
	})
	assert.Equal(t, "healthcare", healthcare.GetIndustry())
	assert.ElementsMatch(t, []string{"HIPAA", "HITECH", "FDA-21-CFR-Part-11", "DEA"}, healthcare.GetRegulations())
	assert.NotEmpty(t, healthcare.GetControls())
	assert.NotEmpty(t, healthcare.GetAudits())
	assert.Len(t, healthcare.GetRiskAssessments(), 1)
	result, err = healthcare.ValidateCompliance(nil)
	require.NoError(t, err)
	assert.True(t, result.Compliant)
	report, err = healthcare.GenerateComplianceReport()
	require.NoError(t, err)
	assert.Equal(t, "healthcare", report.Industry)

	ecommerce := NewEcommerceComplianceTemplate(EcommerceComplianceConfig{
		PCIDSSRequired: true,
		GDPRRequired:   true,
		CCPARequired:   true,
		COPPARequired:  true,
	})
	assert.Equal(t, "ecommerce", ecommerce.GetIndustry())
	assert.ElementsMatch(t, []string{"PCI-DSS", "GDPR", "CCPA", "COPPA"}, ecommerce.GetRegulations())
	assert.NotEmpty(t, ecommerce.GetControls())
	assert.NotEmpty(t, ecommerce.GetAudits())
	assert.NotEmpty(t, ecommerce.GetRiskAssessments())
	result, err = ecommerce.ValidateCompliance(nil)
	require.NoError(t, err)
	assert.True(t, result.Compliant)
	report, err = ecommerce.GenerateComplianceReport()
	require.NoError(t, err)
	assert.Equal(t, "ecommerce", report.Industry)

	government := NewGovernmentComplianceTemplate(GovernmentComplianceConfig{
		FedRAMPLevel:      "Moderate",
		FISMARequired:     true,
		NISTFramework:     "800-53",
		STIGCompliance:    true,
		ATORequired:       true,
		Section508:        true,
		CUIHandling:       true,
		PIIProtection:     true,
		IncidentReporting: true,
	})
	assert.Equal(t, "government", government.GetIndustry())
	regs := government.GetRegulations()
	assert.Contains(t, regs, "FedRAMP")
	assert.Contains(t, regs, "FISMA")
	assert.Contains(t, regs, "NIST-800-53")
	assert.Contains(t, regs, "STIG")
	assert.NotEmpty(t, government.GetControls())
	assert.NotEmpty(t, government.GetAudits())
	assert.NotEmpty(t, government.GetRiskAssessments())
	result, err = government.ValidateCompliance(nil)
	require.NoError(t, err)
	assert.True(t, result.Compliant)
	report, err = government.GenerateComplianceReport()
	require.NoError(t, err)
	assert.Equal(t, "government", report.Industry)
}
