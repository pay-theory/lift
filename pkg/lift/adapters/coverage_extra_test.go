package adapters

import "testing"

func TestEventBridgeAdapter_SourceOverridesTriggerType(t *testing.T) {
	adapter := NewEventBridgeAdapter()

	tests := []struct {
		name        string
		source      string
		wantTrigger TriggerType
	}{
		{name: "aws.s3", source: "aws.s3", wantTrigger: TriggerS3},
		{name: "aws.sqs", source: "aws.sqs", wantTrigger: TriggerSQS},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := map[string]any{
				"source":      tt.source,
				"detail-type": "x",
				"detail":      map[string]any{},
				"time":        "2023-01-01T00:00:00Z",
			}
			req, err := adapter.Adapt(event)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if req.TriggerType != tt.wantTrigger {
				t.Fatalf("expected trigger %s, got %s", tt.wantTrigger, req.TriggerType)
			}
		})
	}
}

func TestS3AndSQSAdapters_CanHandle_NonMap(t *testing.T) {
	if NewS3Adapter().CanHandle("not-a-map") {
		t.Fatal("expected S3 adapter to reject non-map events")
	}
	if NewSQSAdapter().CanHandle("not-a-map") {
		t.Fatal("expected SQS adapter to reject non-map events")
	}
}
