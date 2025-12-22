package lift

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pay-theory/dynamorm"
	"github.com/pay-theory/dynamorm/pkg/core"
	dynamormerrors "github.com/pay-theory/dynamorm/pkg/errors"
	"github.com/pay-theory/dynamorm/pkg/session"

	"github.com/pay-theory/lift/pkg/naming"
)

// DynamoDBConnectionStore implements ConnectionStore using DynamoDB
type DynamoDBConnectionStore struct {
	db       core.ExtendedDB
	ttlHours int
}

// DynamoDBConnectionStoreConfig configures the DynamoDB connection store
type DynamoDBConnectionStoreConfig struct {
	TableName  string
	Region     string
	Endpoint   string
	TTLHours   int // Hours until connection records expire (default: 24)
	MaxRetries int // Max retries for DynamORM (default: 3)
}

const (
	defaultWebSocketConnectionsTableResource = "websocket-connections"
)

var (
	webSocketConnectionsTableNameMu       sync.RWMutex
	webSocketConnectionsTableNameOverride string
)

// NewDynamoDBConnectionStore creates a new DynamoDB-backed connection store
func NewDynamoDBConnectionStore(_ context.Context, config DynamoDBConnectionStoreConfig) (*DynamoDBConnectionStore, error) {
	if config.TTLHours <= 0 {
		config.TTLHours = 24 // Default to 24 hours
	}

	if err := setWebSocketConnectionsTableNameOverride(config.TableName); err != nil {
		return nil, err
	}

	sessionConfig := session.Config{
		Region: config.Region,
	}
	if config.Endpoint != "" {
		sessionConfig.Endpoint = config.Endpoint
	}
	if config.MaxRetries > 0 {
		sessionConfig.MaxRetries = config.MaxRetries
	}

	db, err := dynamorm.New(sessionConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize DynamORM: %w", err)
	}

	return &DynamoDBConnectionStore{
		db:       db,
		ttlHours: config.TTLHours,
	}, nil
}

// NewDynamoDBConnectionStoreWithDB creates a connection store using a provided DynamORM DB.
// This is useful for tests or advanced setups that want to share a single DynamORM instance.
func NewDynamoDBConnectionStoreWithDB(db core.ExtendedDB, config DynamoDBConnectionStoreConfig) (*DynamoDBConnectionStore, error) {
	if db == nil {
		return nil, fmt.Errorf("db is required")
	}
	if config.TTLHours <= 0 {
		config.TTLHours = 24
	}
	if err := setWebSocketConnectionsTableNameOverride(config.TableName); err != nil {
		return nil, err
	}

	return &DynamoDBConnectionStore{
		db:       db,
		ttlHours: config.TTLHours,
	}, nil
}

// dynamormConnectionRecord represents both WebSocket connection records and the
// connection counter item in DynamoDB.
type dynamormConnectionRecord struct {
	Metadata map[string]any `json:"metadata,omitempty"`

	PK string `dynamorm:"pk,attr:PK" json:"-"`
	SK string `dynamorm:"sk,attr:SK" json:"-"`

	// Secondary indexes for common query patterns
	GSI1PK string `dynamorm:"index:gsi1,pk,attr:gsi1pk,omitempty" json:"-"`
	GSI1SK string `dynamorm:"index:gsi1,sk,attr:gsi1sk,omitempty" json:"-"`
	GSI2PK string `dynamorm:"index:gsi2,pk,attr:gsi2pk,omitempty" json:"-"`
	GSI2SK string `dynamorm:"index:gsi2,sk,attr:gsi2sk,omitempty" json:"-"`

	ID        string `json:"id"`
	UserID    string `json:"user_id,omitempty"`
	TenantID  string `json:"tenant_id,omitempty"`
	CreatedAt string `json:"created_at"`

	TTL   int64 `dynamorm:"ttl,omitempty" json:"-"`
	Count int64 `dynamorm:"omitempty" json:"-"`
}

func (*dynamormConnectionRecord) TableName() string {
	if tableName := getWebSocketConnectionsTableNameOverride(); tableName != "" {
		return tableName
	}
	if tableName, ok := naming.ResourceNameFromEnv(defaultWebSocketConnectionsTableResource); ok {
		return tableName
	}
	return defaultWebSocketConnectionsTableResource
}

// Save stores a connection in DynamoDB
func (s *DynamoDBConnectionStore) Save(ctx context.Context, conn *Connection) error {
	if conn == nil {
		return fmt.Errorf("connection is required")
	}
	if conn.ID == "" {
		return fmt.Errorf("connection ID is required")
	}

	record := &dynamormConnectionRecord{
		PK:        fmt.Sprintf("CONNECTION#%s", conn.ID),
		SK:        "CONNECTION",
		ID:        conn.ID,
		UserID:    conn.UserID,
		TenantID:  conn.TenantID,
		CreatedAt: conn.CreatedAt,
		TTL:       time.Now().Add(time.Duration(s.ttlHours) * time.Hour).Unix(),
		Metadata:  conn.Metadata,
	}

	// Set GSI keys if user/tenant IDs are present
	if conn.UserID != "" {
		record.GSI1PK = fmt.Sprintf("USER#%s", conn.UserID)
		record.GSI1SK = fmt.Sprintf("CONNECTION#%s", conn.ID)
	}
	if conn.TenantID != "" {
		record.GSI2PK = fmt.Sprintf("TENANT#%s", conn.TenantID)
		record.GSI2SK = fmt.Sprintf("CONNECTION#%s", conn.ID)
	}

	if err := s.db.WithContext(ctx).Model(record).CreateOrUpdate(); err != nil {
		return fmt.Errorf("failed to save connection: %w", err)
	}

	// Atomically increment the connection counter
	if err := s.incrementConnectionCounter(ctx); err != nil {
		// Log the error but don't fail the connection save
		// The counter is for monitoring, not critical functionality
		fmt.Printf("Warning: failed to increment connection counter: %v\n", err)
	}

	return nil
}

// Get retrieves a connection by ID
func (s *DynamoDBConnectionStore) Get(ctx context.Context, connectionID string) (*Connection, error) {
	if connectionID == "" {
		return nil, fmt.Errorf("connection ID is required")
	}

	pk := fmt.Sprintf("CONNECTION#%s", connectionID)
	var record dynamormConnectionRecord
	err := s.db.WithContext(ctx).Model(&dynamormConnectionRecord{}).
		Where("PK", "=", pk).
		Where("SK", "=", "CONNECTION").
		First(&record)
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrItemNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get connection: %w", err)
	}

	return &Connection{
		ID:        record.ID,
		UserID:    record.UserID,
		TenantID:  record.TenantID,
		CreatedAt: record.CreatedAt,
		Metadata:  record.Metadata,
	}, nil
}

// Delete removes a connection by ID
func (s *DynamoDBConnectionStore) Delete(ctx context.Context, connectionID string) error {
	if connectionID == "" {
		return fmt.Errorf("connection ID is required")
	}

	pk := fmt.Sprintf("CONNECTION#%s", connectionID)
	err := s.db.WithContext(ctx).Model(&dynamormConnectionRecord{}).
		Where("PK", "=", pk).
		Where("SK", "=", "CONNECTION").
		Delete()
	if err != nil {
		return fmt.Errorf("failed to delete connection: %w", err)
	}

	// Atomically decrement the connection counter
	if err := s.decrementConnectionCounter(ctx); err != nil {
		// The counter is for monitoring, not critical functionality
		// Silently ignore counter errors
		_ = err
	}

	return nil
}

// ListByUser retrieves all connections for a user
func (s *DynamoDBConnectionStore) ListByUser(ctx context.Context, userID string) ([]*Connection, error) {
	if userID == "" {
		return nil, fmt.Errorf("user ID is required")
	}

	var records []dynamormConnectionRecord
	err := s.db.WithContext(ctx).Model(&dynamormConnectionRecord{}).
		Index("gsi1").
		Where("GSI1PK", "=", fmt.Sprintf("USER#%s", userID)).
		All(&records)
	if err != nil {
		return nil, fmt.Errorf("failed to query connections by user: %w", err)
	}

	return convertConnectionRecords(records), nil
}

// ListByTenant retrieves all connections for a tenant
func (s *DynamoDBConnectionStore) ListByTenant(ctx context.Context, tenantID string) ([]*Connection, error) {
	if tenantID == "" {
		return nil, fmt.Errorf("tenant ID is required")
	}

	var records []dynamormConnectionRecord
	err := s.db.WithContext(ctx).Model(&dynamormConnectionRecord{}).
		Index("gsi2").
		Where("GSI2PK", "=", fmt.Sprintf("TENANT#%s", tenantID)).
		All(&records)
	if err != nil {
		return nil, fmt.Errorf("failed to query connections by tenant: %w", err)
	}

	return convertConnectionRecords(records), nil
}

// CountActive returns the number of active connections using efficient counter pattern
func (s *DynamoDBConnectionStore) CountActive(ctx context.Context) (int64, error) {
	var counter dynamormConnectionRecord
	err := s.db.WithContext(ctx).Model(&dynamormConnectionRecord{}).
		Where("PK", "=", connectionCounterPK).
		Where("SK", "=", connectionCounterSK).
		First(&counter)
	if err != nil {
		if errors.Is(err, dynamormerrors.ErrItemNotFound) {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get connection counter: %w", err)
	}

	if counter.Count < 0 {
		return 0, nil
	}
	return counter.Count, nil
}

// incrementConnectionCounter atomically increments the connection counter
func (s *DynamoDBConnectionStore) incrementConnectionCounter(ctx context.Context) error {
	return s.updateConnectionCounter(ctx, 1)
}

// decrementConnectionCounter atomically decrements the connection counter
func (s *DynamoDBConnectionStore) decrementConnectionCounter(ctx context.Context) error {
	return s.updateConnectionCounter(ctx, -1)
}

// updateConnectionCounter atomically updates the connection counter by the specified delta
func (s *DynamoDBConnectionStore) updateConnectionCounter(ctx context.Context, delta int64) error {
	return s.db.WithContext(ctx).Model(&dynamormConnectionRecord{}).
		Where("PK", "=", connectionCounterPK).
		Where("SK", "=", connectionCounterSK).
		UpdateBuilder().
		Add("Count", delta).
		Execute()
}

func (s *DynamoDBConnectionStore) CreateTable(_ context.Context) error {
	return s.db.CreateTable(&dynamormConnectionRecord{})
}

const (
	connectionCounterPK = "CONNECTION_COUNTER"
	connectionCounterSK = "COUNTER"
)

func setWebSocketConnectionsTableNameOverride(tableName string) error {
	if tableName == "" {
		return nil
	}

	webSocketConnectionsTableNameMu.Lock()
	defer webSocketConnectionsTableNameMu.Unlock()

	if webSocketConnectionsTableNameOverride != "" && webSocketConnectionsTableNameOverride != tableName {
		return fmt.Errorf("websocket connections table name already set to %q (cannot change to %q)", webSocketConnectionsTableNameOverride, tableName)
	}
	webSocketConnectionsTableNameOverride = tableName
	return nil
}

func getWebSocketConnectionsTableNameOverride() string {
	webSocketConnectionsTableNameMu.RLock()
	defer webSocketConnectionsTableNameMu.RUnlock()
	return webSocketConnectionsTableNameOverride
}

func convertConnectionRecords(records []dynamormConnectionRecord) []*Connection {
	connections := make([]*Connection, 0, len(records))
	for i := range records {
		record := records[i]
		connections = append(connections, &Connection{
			ID:        record.ID,
			UserID:    record.UserID,
			TenantID:  record.TenantID,
			CreatedAt: record.CreatedAt,
			Metadata:  record.Metadata,
		})
	}

	return connections
}
