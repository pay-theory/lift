package lift

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitializeDynamORM_AndConstructors(t *testing.T) {
	setWebSocketTableOverrideForTest(t, "")

	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	db, err := initializeDynamORM("us-east-1", "http://localhost:8000", 1)
	require.NoError(t, err)
	require.NotNil(t, db)

	_, err = NewDynamoDBConnectionStoreWithDB(nil, DynamoDBConnectionStoreConfig{})
	require.Error(t, err)

	_, err = NewDynamoDBSubscriptionStoreWithDB(nil, DynamoDBSubscriptionStoreConfig{})
	require.Error(t, err)

	_, err = NewDynamoDBConnectionStoreWithDB(db, DynamoDBConnectionStoreConfig{
		TableName: "test-table",
		TTLHours:  1,
	})
	require.NoError(t, err)

	_, err = NewDynamoDBSubscriptionStoreWithDB(db, DynamoDBSubscriptionStoreConfig{
		TableName: "test-table",
		TTLHours:  1,
	})
	require.NoError(t, err)

	_, err = NewDynamoDBConnectionStore(context.Background(), DynamoDBConnectionStoreConfig{
		TableName:  "test-table",
		Region:     "us-east-1",
		Endpoint:   "http://localhost:8000",
		TTLHours:   1,
		MaxRetries: 1,
	})
	require.NoError(t, err)

	_, err = NewDynamoDBSubscriptionStore(context.Background(), DynamoDBSubscriptionStoreConfig{
		TableName:  "test-table",
		Region:     "us-east-1",
		Endpoint:   "http://localhost:8000",
		TTLHours:   1,
		MaxRetries: 1,
	})
	require.NoError(t, err)

	require.NotEmpty(t, (&dynamormSubscriptionRecord{}).TableName())
}

func TestDynamoDBStoreConstructors_DefaultTTLAndOverrideErrors(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "test")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "test")
	t.Setenv("AWS_EC2_METADATA_DISABLED", "true")

	db, err := initializeDynamORM("us-east-1", "http://localhost:8000", 1)
	require.NoError(t, err)

	webSocketConnectionsTableNameMu.Lock()
	previous := webSocketConnectionsTableNameOverride
	webSocketConnectionsTableNameOverride = ""
	webSocketConnectionsTableNameMu.Unlock()
	t.Cleanup(func() {
		webSocketConnectionsTableNameMu.Lock()
		webSocketConnectionsTableNameOverride = previous
		webSocketConnectionsTableNameMu.Unlock()
	})

	connStore, err := NewDynamoDBConnectionStoreWithDB(db, DynamoDBConnectionStoreConfig{
		TableName: "test-table",
		TTLHours:  0,
	})
	require.NoError(t, err)
	require.Equal(t, 24, connStore.ttlHours)

	subStore, err := NewDynamoDBSubscriptionStoreWithDB(db, DynamoDBSubscriptionStoreConfig{
		TableName: "test-table",
		TTLHours:  0,
	})
	require.NoError(t, err)
	require.Equal(t, 24, subStore.ttlHours)

	webSocketConnectionsTableNameMu.Lock()
	webSocketConnectionsTableNameOverride = "existing-table"
	webSocketConnectionsTableNameMu.Unlock()

	_, err = NewDynamoDBConnectionStoreWithDB(db, DynamoDBConnectionStoreConfig{TableName: "different-table"})
	require.Error(t, err)

	_, err = NewDynamoDBSubscriptionStoreWithDB(db, DynamoDBSubscriptionStoreConfig{TableName: "different-table"})
	require.Error(t, err)
}
