package lift

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pay-theory/dynamorm/pkg/core"
)

// DynamoDBSubscriptionStore implements SubscriptionStore using the WebSocket connections DynamoDB table.
//
// The table uses a single-table pattern with two item shapes:
//   - Topic → connection mapping:
//     PK = "TOPIC#<tenantID>#<topic>", SK = "CONNECTION#<connectionID>"
//   - Connection → topic reverse mapping (for cleanup):
//     PK = "CONNECTION#<connectionID>", SK = "TOPIC#<tenantID>#<topic>"
//
// This allows fanout without scans and allows disconnect cleanup without scanning by topic.
type DynamoDBSubscriptionStore struct {
	db       core.ExtendedDB
	ttlHours int
}

// DynamoDBSubscriptionStoreConfig configures DynamoDBSubscriptionStore.
//
// TableName should match the WebSocket connections table used by DynamoDBConnectionStore.
type DynamoDBSubscriptionStoreConfig struct {
	TableName  string
	Region     string
	Endpoint   string
	TTLHours   int
	MaxRetries int
}

// NewDynamoDBSubscriptionStore creates a DynamoDB-backed subscription store using its own DynamORM session.
func NewDynamoDBSubscriptionStore(_ context.Context, config DynamoDBSubscriptionStoreConfig) (*DynamoDBSubscriptionStore, error) {
	if config.TTLHours <= 0 {
		config.TTLHours = 24
	}
	if err := setWebSocketConnectionsTableNameOverride(config.TableName); err != nil {
		return nil, err
	}

	db, err := initializeDynamORM(config.Region, config.Endpoint, config.MaxRetries)
	if err != nil {
		return nil, err
	}

	return &DynamoDBSubscriptionStore{
		db:       db,
		ttlHours: config.TTLHours,
	}, nil
}

// NewDynamoDBSubscriptionStoreWithDB creates a subscription store using a provided DynamORM DB.
func NewDynamoDBSubscriptionStoreWithDB(db core.ExtendedDB, config DynamoDBSubscriptionStoreConfig) (*DynamoDBSubscriptionStore, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	if config.TTLHours <= 0 {
		config.TTLHours = 24
	}
	if err := setWebSocketConnectionsTableNameOverride(config.TableName); err != nil {
		return nil, err
	}

	return &DynamoDBSubscriptionStore{
		db:       db,
		ttlHours: config.TTLHours,
	}, nil
}

type dynamormSubscriptionRecord struct {
	PK string `dynamorm:"pk,attr:PK" json:"-"`
	SK string `dynamorm:"sk,attr:SK" json:"-"`

	TenantID     string `json:"tenant_id,omitempty"`
	Topic        string `json:"topic,omitempty"`
	ConnectionID string `json:"connection_id,omitempty"`
	CreatedAt    string `json:"created_at,omitempty"`

	TTL int64 `dynamorm:"ttl,omitempty" json:"-"`
}

func (*dynamormSubscriptionRecord) TableName() string {
	// Reuse the WebSocket connections table naming/override logic.
	return (&dynamormConnectionRecord{}).TableName()
}

func normalizeSubscriptionTopic(topic string) string {
	return strings.TrimSpace(topic)
}

func subscriptionTopicPK(tenantID string, topic string) string {
	return fmt.Sprintf("TOPIC#%s#%s", tenantID, topic)
}

func subscriptionTopicSK(connectionID string) string {
	return fmt.Sprintf("CONNECTION#%s", connectionID)
}

func subscriptionConnectionPK(connectionID string) string {
	return fmt.Sprintf("CONNECTION#%s", connectionID)
}

func subscriptionConnectionSK(tenantID string, topic string) string {
	return fmt.Sprintf("TOPIC#%s#%s", tenantID, topic)
}

func (s *DynamoDBSubscriptionStore) Subscribe(ctx context.Context, tenantID string, connectionID string, topic string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("subscription store is not initialized")
	}
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}
	if connectionID == "" {
		return fmt.Errorf("connectionID is required")
	}

	topic = normalizeSubscriptionTopic(topic)
	if topic == "" {
		return fmt.Errorf("topic is required")
	}

	now := time.Now().UTC()
	ttl := now.Add(time.Duration(s.ttlHours) * time.Hour).Unix()

	// Topic → connection mapping
	forward := &dynamormSubscriptionRecord{
		PK:           subscriptionTopicPK(tenantID, topic),
		SK:           subscriptionTopicSK(connectionID),
		TenantID:     tenantID,
		Topic:        topic,
		ConnectionID: connectionID,
		CreatedAt:    now.Format(time.RFC3339Nano),
		TTL:          ttl,
	}

	// Connection → topic reverse mapping (for cleanup)
	reverse := &dynamormSubscriptionRecord{
		PK:           subscriptionConnectionPK(connectionID),
		SK:           subscriptionConnectionSK(tenantID, topic),
		TenantID:     tenantID,
		Topic:        topic,
		ConnectionID: connectionID,
		CreatedAt:    now.Format(time.RFC3339Nano),
		TTL:          ttl,
	}

	// Use a transaction for atomicity (2 items).
	return s.db.TransactWrite(ctx, func(tx core.TransactionBuilder) error {
		tx.Put(forward)
		tx.Put(reverse)
		return nil
	})
}

func (s *DynamoDBSubscriptionStore) Unsubscribe(ctx context.Context, tenantID string, connectionID string, topic string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("subscription store is not initialized")
	}
	if tenantID == "" {
		return fmt.Errorf("tenantID is required")
	}
	if connectionID == "" {
		return fmt.Errorf("connectionID is required")
	}

	topic = normalizeSubscriptionTopic(topic)
	if topic == "" {
		return fmt.Errorf("topic is required")
	}

	forward := &dynamormSubscriptionRecord{
		PK: subscriptionTopicPK(tenantID, topic),
		SK: subscriptionTopicSK(connectionID),
	}
	reverse := &dynamormSubscriptionRecord{
		PK: subscriptionConnectionPK(connectionID),
		SK: subscriptionConnectionSK(tenantID, topic),
	}

	return s.db.TransactWrite(ctx, func(tx core.TransactionBuilder) error {
		tx.Delete(forward)
		tx.Delete(reverse)
		return nil
	})
}

func (s *DynamoDBSubscriptionStore) ListByTopic(ctx context.Context, tenantID string, topic string) ([]string, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("subscription store is not initialized")
	}
	if tenantID == "" {
		return nil, fmt.Errorf("tenantID is required")
	}

	topic = normalizeSubscriptionTopic(topic)
	if topic == "" {
		return nil, fmt.Errorf("topic is required")
	}

	var records []dynamormSubscriptionRecord
	err := s.db.WithContext(ctx).
		Model(&dynamormSubscriptionRecord{}).
		Where("PK", "=", subscriptionTopicPK(tenantID, topic)).
		All(&records)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions by topic: %w", err)
	}

	out := make([]string, 0, len(records))
	for i := range records {
		if records[i].ConnectionID == "" {
			continue
		}
		out = append(out, records[i].ConnectionID)
	}

	return out, nil
}

func (s *DynamoDBSubscriptionStore) ListByConnection(ctx context.Context, connectionID string) ([]Subscription, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("subscription store is not initialized")
	}
	if connectionID == "" {
		return nil, fmt.Errorf("connectionID is required")
	}

	var records []dynamormSubscriptionRecord
	err := s.db.WithContext(ctx).
		Model(&dynamormSubscriptionRecord{}).
		Where("PK", "=", subscriptionConnectionPK(connectionID)).
		Where("SK", "BEGINS_WITH", "TOPIC#").
		All(&records)
	if err != nil {
		return nil, fmt.Errorf("failed to query subscriptions by connection: %w", err)
	}

	out := make([]Subscription, 0, len(records))
	for i := range records {
		record := records[i]
		if record.Topic == "" || record.TenantID == "" {
			continue
		}
		out = append(out, Subscription{
			TenantID:     record.TenantID,
			Topic:        record.Topic,
			ConnectionID: connectionID,
		})
	}

	return out, nil
}

func (s *DynamoDBSubscriptionStore) DeleteByConnection(ctx context.Context, connectionID string) (int, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("subscription store is not initialized")
	}
	if connectionID == "" {
		return 0, fmt.Errorf("connectionID is required")
	}

	subs, err := s.ListByConnection(ctx, connectionID)
	if err != nil {
		return 0, err
	}
	if len(subs) == 0 {
		return 0, nil
	}

	keys := make([]any, 0, len(subs)*2)
	for _, sub := range subs {
		// Reverse mapping
		keys = append(keys, &dynamormSubscriptionRecord{
			PK: subscriptionConnectionPK(connectionID),
			SK: subscriptionConnectionSK(sub.TenantID, sub.Topic),
		})
		// Forward mapping
		keys = append(keys, &dynamormSubscriptionRecord{
			PK: subscriptionTopicPK(sub.TenantID, sub.Topic),
			SK: subscriptionTopicSK(connectionID),
		})
	}

	deleted := 0
	const batchLimit = 25
	for start := 0; start < len(keys); start += batchLimit {
		end := start + batchLimit
		if end > len(keys) {
			end = len(keys)
		}

		if err := s.db.WithContext(ctx).
			Model(&dynamormSubscriptionRecord{}).
			BatchWrite(nil, keys[start:end]); err != nil {
			return deleted, fmt.Errorf("failed to batch delete subscriptions: %w", err)
		}
		deleted += end - start
	}

	return deleted, nil
}
