package security

import (
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recordingLogger struct {
	mu    sync.Mutex
	errs  []string
	infos []string
	warns []string
}

func (l *recordingLogger) Error(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.errs = append(l.errs, msg)
}

func (l *recordingLogger) Info(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.infos = append(l.infos, msg)
}

func (l *recordingLogger) Warn(msg string, _ ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.warns = append(l.warns, msg)
}

type testLiftContext struct {
	values        map[string]any
	userID        string
	tenantID      string
	clientIP      string
	logger        Logger
	dataAccessLog []string
	mu            sync.Mutex
}

func newTestLiftContext(userID, tenantID string, logger Logger) *testLiftContext {
	return &testLiftContext{
		values:        make(map[string]any),
		userID:        userID,
		tenantID:      tenantID,
		clientIP:      "127.0.0.1",
		logger:        logger,
		dataAccessLog: []string{},
	}
}

func (t *testLiftContext) Set(key string, value any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.values[key] = value
}

func (t *testLiftContext) Get(key string) any {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.values[key]
}

func (t *testLiftContext) UserID() string { return t.userID }
func (t *testLiftContext) TenantID() string {
	return t.tenantID
}
func (t *testLiftContext) ClientIP() string { return t.clientIP }
func (t *testLiftContext) Logger() Logger   { return t.logger }
func (t *testLiftContext) GetDataAccessLog() []string {
	return t.dataAccessLog
}

type recordingEnhancedAuditor struct {
	startSOC2Calls    int
	startAuditCalls   int
	logControlsCalls  int
	logGDPRCalls      int
	logTestCalls      int
	completeSOC2Calls int

	logControlsErr error
	logGDPRErr     error
	logTestErr     error
	completeErr    error

	lastSOC2AuditID string
	lastAuditID     string
}

func (a *recordingEnhancedAuditor) StartAudit(LiftContext) string {
	a.startAuditCalls++
	a.lastAuditID = "audit-id"
	return a.lastAuditID
}
func (a *recordingEnhancedAuditor) LogRequest(string, *AuditRequest) error        { return nil }
func (a *recordingEnhancedAuditor) LogResponse(string, *AuditResponse) error      { return nil }
func (a *recordingEnhancedAuditor) LogDataAccess(string, *DataAccessLog) error    { return nil }
func (a *recordingEnhancedAuditor) LogSecurityEvent(string, *SecurityEvent) error { return nil }

func (a *recordingEnhancedAuditor) StartSOC2Audit(LiftContext) string {
	a.startSOC2Calls++
	a.lastSOC2AuditID = "soc2-id"
	return a.lastSOC2AuditID
}

func (a *recordingEnhancedAuditor) LogSecurityControls(string, *SOC2Controls) error {
	a.logControlsCalls++
	return a.logControlsErr
}

func (a *recordingEnhancedAuditor) LogGDPREvent(string, *GDPREvent) error {
	a.logGDPRCalls++
	return a.logGDPRErr
}

func (a *recordingEnhancedAuditor) LogComplianceTest(string, *ComplianceTestResult) error {
	a.logTestCalls++
	return a.logTestErr
}

func (a *recordingEnhancedAuditor) LogDataProcessing(string, *DataProcessingLog) error { return nil }

func (a *recordingEnhancedAuditor) CompleteSOC2Audit(string, any, error) error {
	a.completeSOC2Calls++
	return a.completeErr
}

type stubAdvancedValidator struct {
	soc2Result *ComplianceResult
	soc2Err    error
}

func (v *stubAdvancedValidator) ValidateRequest(LiftContext, string) (*ComplianceResult, error) {
	return &ComplianceResult{Compliant: true}, nil
}
func (v *stubAdvancedValidator) ValidateDataAccess(LiftContext, string) (*ComplianceResult, error) {
	return &ComplianceResult{Compliant: true}, nil
}
func (v *stubAdvancedValidator) ValidateRegion(LiftContext, string) (*ComplianceResult, error) {
	return &ComplianceResult{Compliant: true}, nil
}
func (v *stubAdvancedValidator) ValidateSOC2Controls(LiftContext, *SOC2Controls) (*ComplianceResult, error) {
	return v.soc2Result, v.soc2Err
}
func (v *stubAdvancedValidator) ValidateGDPRCompliance(LiftContext, string, any) (*ComplianceResult, error) {
	return &ComplianceResult{Compliant: true}, nil
}
func (v *stubAdvancedValidator) ValidateDataProcessingBasis(LiftContext, string) (*ComplianceResult, error) {
	return &ComplianceResult{Compliant: true}, nil
}
func (v *stubAdvancedValidator) ValidateDataMinimization(LiftContext, any) (*ComplianceResult, error) {
	return &ComplianceResult{Compliant: true}, nil
}
func (v *stubAdvancedValidator) ValidateConsentRequirements(LiftContext, *ConsentData) (*ComplianceResult, error) {
	return &ComplianceResult{Compliant: true}, nil
}

type stubTemplate struct {
	controls []ComplianceControl
}

func (s *stubTemplate) GetIndustry() string                                 { return "stub" }
func (s *stubTemplate) GetRegulations() []string                            { return nil }
func (s *stubTemplate) GetControls() []ComplianceControl                    { return s.controls }
func (s *stubTemplate) GetAudits() []AuditRequirement                       { return nil }
func (s *stubTemplate) ApplyToFramework(*EnhancedComplianceFramework) error { return nil }

func TestEnhancedComplianceFramework_SOC2TypeII(t *testing.T) {
	t.Parallel()

	t.Run("disabled passthrough", func(t *testing.T) {
		t.Parallel()

		ecf := NewEnhancedComplianceFramework("SOC2-TypeII", EnhancedComplianceConfig{
			SOC2TypeII: SOC2TypeIIConfig{Enabled: false},
		})

		called := false
		handler := LiftHandlerFunc(func(LiftContext) error {
			called = true
			return nil
		})
		logger := &recordingLogger{}
		ctx := newTestLiftContext("u1", "t1", logger)

		mw := ecf.SOC2TypeII()
		require.NoError(t, mw(handler).Handle(ctx))
		assert.True(t, called)
	})

	t.Run("enabled runs auditor and validator", func(t *testing.T) {
		t.Parallel()

		auditor := &recordingEnhancedAuditor{
			logControlsErr: errors.New("log controls failed"),
			completeErr:    errors.New("complete failed"),
		}
		validator := &stubAdvancedValidator{
			soc2Result: &ComplianceResult{
				Compliant: false,
				Violations: []ComplianceViolation{
					{RuleID: "C-1", Severity: riskLevelHigh, Description: "bad"},
					{RuleID: "C-2", Severity: riskLevelCritical, Description: "worse"},
				},
			},
		}

		ecf := NewEnhancedComplianceFramework("SOC2-TypeII", EnhancedComplianceConfig{
			SOC2TypeII: SOC2TypeIIConfig{Enabled: true},
		})
		ecf.SetEnhancedAuditor(auditor)
		ecf.SetAdvancedValidator(validator)

		logger := &recordingLogger{}
		ctx := newTestLiftContext("u1", "t1", logger)

		handlerErr := errors.New("handler failed")
		handler := LiftHandlerFunc(func(LiftContext) error { return handlerErr })

		err := ecf.SOC2TypeII()(handler).Handle(ctx)
		require.ErrorIs(t, err, handlerErr)

		assert.Equal(t, 1, auditor.startSOC2Calls)
		assert.Equal(t, 1, auditor.logControlsCalls)
		assert.Equal(t, 1, auditor.completeSOC2Calls)
		assert.NotEmpty(t, logger.warns)
	})

	t.Run("validator error is logged", func(t *testing.T) {
		t.Parallel()

		validator := &stubAdvancedValidator{soc2Err: errors.New("validate failed")}
		ecf := NewEnhancedComplianceFramework("SOC2-TypeII", EnhancedComplianceConfig{
			SOC2TypeII: SOC2TypeIIConfig{Enabled: true},
		})
		ecf.SetAdvancedValidator(validator)

		logger := &recordingLogger{}
		ctx := newTestLiftContext("u1", "t1", logger)
		handler := LiftHandlerFunc(func(LiftContext) error { return nil })

		require.NoError(t, ecf.SOC2TypeII()(handler).Handle(ctx))
		assert.NotEmpty(t, logger.errs)
	})
}

func TestEnhancedComplianceFramework_GDPRPrivacyAndTemplatesAndDeletion(t *testing.T) {
	t.Parallel()

	logger := &recordingLogger{}
	ctx := newTestLiftContext("u1", "t1", logger)

	t.Run("gdpr disabled passthrough", func(t *testing.T) {
		t.Parallel()

		ecf := NewEnhancedComplianceFramework("GDPR", EnhancedComplianceConfig{GDPR: GDPRConfig{Enabled: false}})
		called := false
		handler := LiftHandlerFunc(func(LiftContext) error { called = true; return nil })
		require.NoError(t, ecf.GDPRPrivacy()(handler).Handle(ctx))
		assert.True(t, called)
	})

	t.Run("gdpr enabled logs event and continues", func(t *testing.T) {
		t.Parallel()

		auditor := &recordingEnhancedAuditor{logGDPRErr: errors.New("log failed")}
		ecf := NewEnhancedComplianceFramework("GDPR", EnhancedComplianceConfig{
			GDPR: GDPRConfig{Enabled: true, DataMinimization: true},
		})
		ecf.SetEnhancedAuditor(auditor)

		called := false
		handler := LiftHandlerFunc(func(LiftContext) error { called = true; return nil })
		require.NoError(t, ecf.GDPRPrivacy()(handler).Handle(ctx))
		assert.True(t, called)
		assert.Equal(t, 1, auditor.startAuditCalls)
		assert.Equal(t, 1, auditor.logGDPRCalls)
		assert.NotEmpty(t, logger.errs)
	})

	t.Run("apply industry template builds middlewares", func(t *testing.T) {
		t.Parallel()

		auditor := &recordingEnhancedAuditor{logTestErr: errors.New("test log failed")}
		ecf := NewEnhancedComplianceFramework("NIST", EnhancedComplianceConfig{})
		ecf.SetEnhancedAuditor(auditor)

		ecf.AddIndustryTemplate("demo", &stubTemplate{controls: []ComplianceControl{
			{ID: "c1", Name: "auto", Framework: "NIST", Automated: true},
			{ID: "c2", Name: "manual", Framework: "NIST", Automated: false},
		}})

		mws, err := ecf.ApplyIndustryTemplate("demo")
		require.NoError(t, err)
		require.Len(t, mws, 2)

		called := false
		handler := LiftHandlerFunc(func(LiftContext) error { called = true; return nil })
		var wrapped LiftHandler = handler
		for i := len(mws) - 1; i >= 0; i-- {
			wrapped = mws[i](wrapped)
		}

		require.NoError(t, wrapped.Handle(ctx))
		assert.True(t, called)
		assert.Equal(t, 1, auditor.logTestCalls)
		assert.NotEmpty(t, logger.errs)

		_, err = ecf.ApplyIndustryTemplate("missing")
		require.Error(t, err)
	})

	t.Run("data deletion request", func(t *testing.T) {
		t.Parallel()

		auditor := &recordingEnhancedAuditor{}
		ecf := NewEnhancedComplianceFramework("GDPR", EnhancedComplianceConfig{})
		ecf.SetEnhancedAuditor(auditor)

		ctx := newTestLiftContext("user", "tenant", &recordingLogger{})
		ctx.Set("email", "user@example.com")
		ctx.Set("retain_for_legal", true)
		ctx.Set("deletion_reason", "cleanup")

		require.NoError(t, ecf.handleDataDeletion(ctx))
		assert.Equal(t, 2, auditor.startAuditCalls)
		assert.Equal(t, 2, auditor.logGDPRCalls)

		ctxEmpty := newTestLiftContext("", "tenant", &recordingLogger{})
		require.Error(t, ecf.handleDataDeletion(ctxEmpty))

		ctxBadScope := newTestLiftContext("user", "tenant", &recordingLogger{})
		ctxBadScope.Set("erasure_scope", []string{})
		require.Error(t, ecf.handleDataDeletion(ctxBadScope))
	})
}

func TestEnhancedComplianceFramework_HelperMethods(t *testing.T) {
	t.Parallel()

	ecf := NewEnhancedComplianceFramework("GDPR", EnhancedComplianceConfig{})
	logger := &recordingLogger{}
	ctx := newTestLiftContext("u1", "t1", logger)

	assert.Equal(t, "u1@example.com", ecf.extractEmailFromContext(ctx))
	ctx.Set("email", "test@example.com")
	assert.Equal(t, "test@example.com", ecf.extractEmailFromContext(ctx))

	scope := ecf.extractErasureScopeFromContext(ctx)
	assert.NotEmpty(t, scope)
	ctx.Set("erasure_scope", []string{"profile"})
	assert.Equal(t, []string{"profile"}, ecf.extractErasureScopeFromContext(ctx))

	assert.False(t, ecf.shouldRetainForLegal(ctx))
	ctx.Set("retain_for_legal", true)
	assert.True(t, ecf.shouldRetainForLegal(ctx))

	assert.Equal(t, "user_request", ecf.extractDeletionReason(ctx))
	ctx.Set("deletion_reason", "x")
	assert.Equal(t, "x", ecf.extractDeletionReason(ctx))

	require.Error(t, ecf.validateErasureRequest(&DataErasureRequest{}))
	require.Error(t, ecf.validateErasureRequest(&DataErasureRequest{DataAccessRequest: DataAccessRequest{DataSubjectID: "u"}}))
	require.NoError(t, ecf.validateErasureRequest(&DataErasureRequest{DataAccessRequest: DataAccessRequest{DataSubjectID: "u"}, ErasureScope: []string{"a"}}))

	assert.Empty(t, ecf.getDataDeletionProviders())
	assert.False(t, ecf.hasRequiredProviderFailures(nil))
	assert.True(t, ecf.hasRequiredProviderFailures([]error{errors.New("x")}))

	erased := ecf.collectErasedDataCategories([]DataDeletionResult{
		{DeletedDataTypes: []string{"a", "b"}},
		{DeletedDataTypes: []string{"b", "c"}},
	})
	assert.ElementsMatch(t, []string{"a", "b", "c"}, erased)

	retained := ecf.collectRetainedDataCategories([]DataDeletionResult{
		{RetainedDataTypes: []string{"x"}},
		{RetainedDataTypes: []string{"x", "y"}},
	})
	assert.ElementsMatch(t, []string{"x", "y"}, retained)

	assert.Equal(t, "Legal retention requirements", ecf.buildRetentionReason(true, nil))
	reason := ecf.buildRetentionReason(false, []DataDeletionResult{{RetentionReasons: []string{"r1", "r2"}}})
	assert.Contains(t, reason, "Provider-specific retention")

	assert.Equal(t, 6, ecf.calculateTotalDeletedRecords([]DataDeletionResult{{DeletedRecords: 1}, {DeletedRecords: 5}}))

	assert.Equal(t, "success", ecf.getStatusFromError(nil))
	assert.Equal(t, "error", ecf.getStatusFromError(errors.New("x")))
}
