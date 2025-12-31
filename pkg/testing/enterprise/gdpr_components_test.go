package enterprise

import (
	"context"
	"testing"
	"time"
)

func TestDataMapper_MapAndGet(t *testing.T) {
	mapper := NewDataMapper()

	mapping := &DataMapping{
		ID:          "m1",
		DataType:    IdentifyingData,
		Source:      "src",
		Destination: "dst",
		Purpose:     "marketing",
	}
	if err := mapper.MapData(context.Background(), mapping); err != nil {
		t.Fatalf("MapData error: %v", err)
	}
	if mapping.LastUpdated.IsZero() {
		t.Fatalf("expected LastUpdated to be set")
	}
	got := mapper.GetDataMappings(context.Background())
	if got["m1"] == nil {
		t.Fatalf("expected mapping to be stored")
	}
}

func TestConsentManager_GrantWithdrawGet(t *testing.T) {
	manager := NewConsentManager()

	if _, err := manager.GetConsent(context.Background(), "missing"); err == nil {
		t.Fatalf("GetConsent expected error")
	}
	if err := manager.WithdrawConsent(context.Background(), "missing"); err == nil {
		t.Fatalf("WithdrawConsent expected error")
	}

	record := &LocalConsentRecord{
		ID:      "c1",
		Purpose: "marketing",
	}
	if err := manager.GrantConsent(context.Background(), record); err != nil {
		t.Fatalf("GrantConsent error: %v", err)
	}
	if !record.Granted {
		t.Fatalf("expected Granted=true")
	}
	if _, err := manager.GetConsent(context.Background(), "c1"); err != nil {
		t.Fatalf("GetConsent(c1) error: %v", err)
	}
	if err := manager.WithdrawConsent(context.Background(), "c1"); err != nil {
		t.Fatalf("WithdrawConsent(c1) error: %v", err)
	}
	if record.WithdrawnAt == nil {
		t.Fatalf("expected WithdrawnAt to be set")
	}
}

func TestTransferValidator_ValidateTransfer(t *testing.T) {
	validator := NewTransferValidator()

	res, err := validator.ValidateTransfer(context.Background(), &DataTransfer{
		ID:          "t1",
		Destination: "XX",
		Mechanism:   AdequacyDecision,
	})
	if err != nil {
		t.Fatalf("ValidateTransfer unknown country error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected invalid transfer for unknown country")
	}

	res, err = validator.ValidateTransfer(context.Background(), &DataTransfer{
		ID:          "t2",
		Destination: "US",
		Mechanism:   TransferMechanism("unknown"),
		Safeguards:  nil,
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("ValidateTransfer error: %v", err)
	}
	if res.Valid {
		t.Fatalf("expected invalid transfer for unknown mechanism and missing safeguards")
	}

	res, err = validator.ValidateTransfer(context.Background(), &DataTransfer{
		ID:          "t3",
		Destination: "CA",
		Mechanism:   AdequacyDecision,
		Timestamp:   time.Now(),
	})
	if err != nil {
		t.Fatalf("ValidateTransfer error: %v", err)
	}
	if !res.Valid {
		t.Fatalf("expected valid transfer to adequate country")
	}
}

