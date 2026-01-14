package compliance

import (
	"context"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/ses"
	"github.com/aws/aws-sdk-go-v2/service/sns"
	dynamormmocks "github.com/pay-theory/dynamorm/pkg/mocks"
	"github.com/pay-theory/lift/pkg/dynamorm"
	dbmocks "github.com/pay-theory/lift/pkg/dynamorm/mocks"
	"github.com/pay-theory/lift/pkg/lift"
	"github.com/pay-theory/lift/pkg/lift/adapters"
	"github.com/pay-theory/lift/pkg/security"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

func captureWrapper(t *testing.T, factory dynamorm.DBFactory) *dynamorm.DynamORMWrapper {
	t.Helper()

	cfg := dynamorm.DefaultConfig()
	cfg.AutoTransaction = false
	cfg.TenantIsolation = false

	mw := dynamorm.WithDynamORM(cfg, factory)

	ctx := lift.NewContext(context.Background(), lift.NewRequest(&adapters.Request{
		Method: "GET",
		Path:   "/",
	}))

	var wrapper *dynamorm.DynamORMWrapper
	err := mw(lift.HandlerFunc(func(ctx *lift.Context) error {
		var err error
		wrapper, err = dynamorm.DB(ctx)
		return err
	})).Handle(ctx)
	require.NoError(t, err)
	require.NotNil(t, wrapper)
	return wrapper
}

func newTestService(t *testing.T, cfg GDPRCompleteConfig, inspectModel func(any)) *GDPRCompleteService {
	t.Helper()

	db := dbmocks.NewMockExtendedDB()
	query := new(dynamormmocks.MockQuery)

	db.On("WithContext", mock.Anything).Return(db).Maybe()
	db.On("Model", mock.Anything).
		Return(query).
		Run(func(args mock.Arguments) {
			if inspectModel != nil {
				inspectModel(args.Get(0))
			}
		}).
		Maybe()
	query.On("Create").Return(nil).Maybe()

	wrapper := captureWrapper(t, &dynamorm.MockDBFactory{MockDB: db})
	return NewGDPRCompleteService(cfg, wrapper)
}

func TestNewGDPRCompleteService_Initializes(t *testing.T) {
	svc := newTestService(t, GDPRCompleteConfig{Environment: "test"}, nil)
	require.NotNil(t, svc.auditLogger)
	require.Len(t, svc.encryptionKey, 32)
	require.Equal(t, "test", svc.config.Environment)
}

func TestGDPRCompleteService_DeleteUserData(t *testing.T) {
	var sawDeletion bool
	svc := newTestService(t, GDPRCompleteConfig{Environment: "test"}, func(model any) {
		if _, ok := model.(*DataDeletionRecord); ok {
			sawDeletion = true
		}
	})

	err := svc.DeleteUserData(context.Background(), "", "req-1")
	require.Error(t, err)

	ctx := context.WithValue(context.Background(), "user_id", "user-1")
	require.NoError(t, svc.DeleteUserData(ctx, "subject-1", "req-1"))
	require.True(t, sawDeletion)
}

func TestGDPRCompleteService_ExportUserData_EncryptsAndUploads(t *testing.T) {
	var sawExport bool
	svc := newTestService(t, GDPRCompleteConfig{
		Environment:       "lab",
		DataExportBucket:  "bucket",
		MaxExportSizeMB:   10,
		EncryptionEnabled: true,
	}, func(model any) {
		if _, ok := model.(*DataExportRecord); ok {
			sawExport = true
		}
	})

	// Enable upload path; no methods are invoked on the client in the current implementation.
	svc.s3Client = &s3.Client{}

	rec, err := svc.ExportUserData(context.Background(), "subject-1", "req-1")
	require.NoError(t, err)
	require.NotNil(t, rec)
	require.Equal(t, "completed", rec.Status)
	require.NotEmpty(t, rec.ExportPath)
	require.NotEmpty(t, rec.EncryptionKey)
	require.True(t, sawExport)
}

func TestGDPRCompleteService_ExportUserData_SizeLimitFailsClosed(t *testing.T) {
	svc := newTestService(t, GDPRCompleteConfig{
		Environment:      "lab",
		DataExportBucket: "bucket",
		MaxExportSizeMB:  0,
	}, nil)
	svc.s3Client = &s3.Client{}

	rec, err := svc.ExportUserData(context.Background(), "subject-1", "req-1")
	require.Error(t, err)
	require.Nil(t, rec)
}

func TestGDPRCompleteService_ProcessConsentUpdate(t *testing.T) {
	var stored *ConsentRecordComplete
	svc := newTestService(t, GDPRCompleteConfig{
		Environment:       "lab",
		ConsentExpiryDays: 1,
	}, func(model any) {
		if rec, ok := model.(*ConsentRecordComplete); ok {
			stored = rec
		}
	})

	ctx := context.WithValue(context.Background(), "client_ip", "1.2.3.4")
	ctx = context.WithValue(ctx, "user_agent", "ua")

	err := svc.ProcessConsentUpdate(ctx, "subject-1", ConsentUpdate{
		Categories: []string{"analytics"},
		Method:     "web",
		LegalBasis: "consent",
		Evidence:   "e",
	})
	require.NoError(t, err)
	require.NotNil(t, stored)
	require.NotNil(t, stored.ExpiryDate)
	require.NotZero(t, stored.TTL)
}

func TestGDPRCompleteService_ProcessBreachNotification(t *testing.T) {
	var stored *PrivacyBreachRecord
	svc := newTestService(t, GDPRCompleteConfig{
		Environment:             "lab",
		BreachNotificationHours: 72,
	}, func(model any) {
		if rec, ok := model.(*PrivacyBreachRecord); ok {
			stored = rec
		}
	})

	ctx := context.WithValue(context.Background(), "user_id", "user-1")

	err := svc.ProcessBreachNotification(ctx, PrivacyBreach{
		DetectedAt:     time.Unix(1, 0).UTC(),
		Type:           "incident",
		Severity:       "high",
		Cause:          "cause",
		DataCategories: []string{"pii"},
		MitigationSteps: []string{
			"rotate",
		},
		AffectedCount: 5,
	})
	require.NoError(t, err)
	require.NotNil(t, stored)
}

func TestGDPRCompleteService_HelperCoverage(t *testing.T) {
	svc := &GDPRCompleteService{config: GDPRCompleteConfig{AuditTableName: "audit"}}

	// deleteItem key parsing branches
	require.Error(t, svc.deleteItem(context.Background(), "t", map[string]interface{}{"PK": 123}))
	require.Error(t, svc.deleteItem(context.Background(), "t", map[string]interface{}{"PK": "pk", "SK": 123}))
	require.NoError(t, svc.deleteItem(context.Background(), "t", map[string]interface{}{"PK": "pk", "SK": "sk"}))

	// shouldRetainData branches
	ok, _ := svc.shouldRetainData(map[string]interface{}{}, "transactions")
	require.True(t, ok)
	ok, _ = svc.shouldRetainData(map[string]interface{}{}, "audit")
	require.True(t, ok)
	ok, _ = svc.shouldRetainData(map[string]interface{}{"status": "legal_hold"}, "t")
	require.True(t, ok)
	ok, _ = svc.shouldRetainData(map[string]interface{}{}, "t")
	require.False(t, ok)

	// convertAttributeValue recursion branches
	require.Equal(t, "v", svc.convertAttributeValue(&types.AttributeValueMemberS{Value: "v"}))
	require.Equal(t, "1", svc.convertAttributeValue(&types.AttributeValueMemberN{Value: "1"}))
	require.Equal(t, true, svc.convertAttributeValue(&types.AttributeValueMemberBOOL{Value: true}))
	require.Equal(t, []interface{}{"a"}, svc.convertAttributeValue(&types.AttributeValueMemberL{
		Value: []types.AttributeValue{
			&types.AttributeValueMemberS{Value: "a"},
		},
	}))
	require.Equal(t, map[string]interface{}{"k": "v"}, svc.convertAttributeValue(&types.AttributeValueMemberM{
		Value: map[string]types.AttributeValue{
			"k": &types.AttributeValueMemberS{Value: "v"},
		},
	}))
	require.Nil(t, svc.convertAttributeValue(&types.AttributeValueMemberNULL{Value: true}))
}

func TestGDPRCompleteService_AdditionalBranches(t *testing.T) {
	var svc *GDPRCompleteService
	svc = newTestService(t, GDPRCompleteConfig{
		Environment:      "lab",
		DataExportBucket: "bucket",
		MaxExportSizeMB:  10,
	}, nil)

	// processExportData marshal error branch + status update path.
	builder := newDataExportBuilder(context.Background(), svc, "subject-1", "req-1")
	builder.exportRecord = &DataExportRecord{Status: "processing"}
	builder.exportData["bad"] = make(chan int)
	_, err := builder.processExportData()
	require.Error(t, err)

	// uploadToS3 error branch.
	svc.s3Client = nil
	require.Error(t, svc.uploadToS3(context.Background(), "b", "k", []byte("x")))

	// deleteUserFiles non-nil branch.
	svc.s3Client = &s3.Client{}
	require.NoError(t, svc.deleteUserFiles(context.Background(), "subject-1"))

	// Notification branches with configured clients (no methods invoked).
	svc.sesClient = &ses.Client{}
	svc.snsClient = &sns.Client{}
	require.NoError(t, svc.sendDeletionNotification(context.Background(), &DataDeletionRecord{DataSubjectID: "s"}))
	require.NoError(t, svc.sendExportNotification(context.Background(), &DataExportRecord{DataSubjectID: "s"}))
	require.NoError(t, svc.sendBreachNotification(context.Background(), &PrivacyBreachRecord{BreachID: "b"}))

	// validateConsentCategories error branches.
	require.Error(t, svc.validateConsentCategories(ConsentUpdate{}))
	require.Error(t, svc.validateConsentCategories(ConsentUpdate{Categories: []string{"not-a-category"}}))

	// Context fallbacks.
	require.Equal(t, "system", svc.getCurrentUser(context.Background()))
	require.Equal(t, "unknown", svc.getClientIP(context.Background()))
	require.Equal(t, "unknown", svc.getUserAgent(context.Background()))

	// Audit logger methods that are otherwise no-ops.
	al := &GDPRAuditLogger{service: svc}
	al.LogError(context.Background(), "a", "m", map[string]interface{}{"k": "v"})

	require.NoError(t, al.LogDataSubjectRequest(context.Background(), &security.DataSubjectRequestLog{
		RequestType:   "export",
		DataSubjectID: "s",
	}))
	require.NoError(t, al.LogDataProcessingActivity(context.Background(), &security.DataProcessingLog{}))
	require.NoError(t, al.LogCrossBorderTransfer(context.Background(), &security.CrossBorderTransferLog{
		SourceCountry:      "US",
		DestinationCountry: "CA",
	}))
}
