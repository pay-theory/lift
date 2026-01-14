package enterprise

import (
	"context"
	"testing"
)

func TestGDPRComplianceTester_RightsAndAudit(t *testing.T) {
	tester := NewGDPRComplianceTester(nil)

	if err := tester.TestRightToAccess(context.Background(), ""); err == nil {
		t.Fatalf("TestRightToAccess expected error for empty userID")
	}
	if err := tester.TestRightToAccess(context.Background(), "u1"); err != nil {
		t.Fatalf("TestRightToAccess error: %v", err)
	}

	if err := tester.TestRightToRectification(context.Background(), "", map[string]any{"name": "x"}); err == nil {
		t.Fatalf("TestRightToRectification expected error for empty userID")
	}
	if err := tester.TestRightToRectification(context.Background(), "u1", nil); err == nil {
		t.Fatalf("TestRightToRectification expected error for empty updates")
	}

	// Force non-map data path.
	tester.dataStore["u1"] = "not a map"
	if err := tester.TestRightToRectification(context.Background(), "u1", map[string]any{"name": "x"}); err == nil {
		t.Fatalf("TestRightToRectification expected error for non-map user data")
	}
	tester.dataStore["u1"] = map[string]any{"user_id": "u1"}
	if err := tester.TestRightToRectification(context.Background(), "u1", map[string]any{"name": "x"}); err != nil {
		t.Fatalf("TestRightToRectification error: %v", err)
	}

	if err := tester.TestRightToErasure(context.Background(), ""); err == nil {
		t.Fatalf("TestRightToErasure expected error for empty userID")
	}
	if err := tester.TestRightToErasure(context.Background(), "u2"); err != nil {
		t.Fatalf("TestRightToErasure error: %v", err)
	}

	if err := tester.TestRightToDataPortability(context.Background(), ""); err == nil {
		t.Fatalf("TestRightToDataPortability expected error for empty userID")
	}
	if err := tester.TestRightToDataPortability(context.Background(), "u3"); err != nil {
		t.Fatalf("TestRightToDataPortability error: %v", err)
	}

	if err := tester.TestRightToObject(context.Background(), "", "marketing"); err == nil {
		t.Fatalf("TestRightToObject expected error for empty userID")
	}
	if err := tester.TestRightToObject(context.Background(), "u4", ""); err == nil {
		t.Fatalf("TestRightToObject expected error for empty processing type")
	}
	if err := tester.TestRightToObject(context.Background(), "u4", "marketing"); err != nil {
		t.Fatalf("TestRightToObject error: %v", err)
	}

	if _, err := tester.GetAuditTrail(context.Background(), ""); err == nil {
		t.Fatalf("GetAuditTrail expected error for empty userID")
	}
	if trail, err := tester.GetAuditTrail(context.Background(), "u1"); err != nil || trail == nil || trail.UserID != "u1" {
		t.Fatalf("GetAuditTrail trail=%#v err=%v", trail, err)
	}
}
