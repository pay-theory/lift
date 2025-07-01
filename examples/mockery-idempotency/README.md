# Mockery Payment Service - Idempotency Example

This example demonstrates how the Mockery team can implement idempotent payment processing using Lift's idempotency middleware.

## Key Features Demonstrated

1. **Automatic Idempotency**: Payment handlers don't need any idempotency logic
2. **Retry Safety**: Network retries won't cause duplicate charges
3. **Concurrent Request Handling**: Prevents race conditions
4. **Error Caching**: Even failed payments are cached to prevent repeated attempts
5. **Production-Ready Patterns**: Shows logging, monitoring, and error handling

## Running the Example

```bash
cd examples/mockery-idempotency
go run main.go
```

The example will:
1. Start a payment service with idempotency enabled
2. Simulate various payment scenarios:
   - Normal payment with network retry
   - Multiple payments from same customer
   - Concurrent duplicate requests
3. Show how idempotency prevents double charges

## API Endpoints

### POST /api/v1/payments
Process a payment (idempotent)

**Request:**
```bash
curl -X POST http://localhost:8080/api/v1/payments \
  -H "Content-Type: application/json" \
  -H "Idempotency-Key: payment-unique-key-123" \
  -d '{
    "amount": 99.99,
    "currency": "USD",
    "customer_id": "cust-123",
    "description": "Monthly subscription"
  }'
```

**Response:**
```json
{
  "transaction_id": "txn_1234567890",
  "status": "completed",
  "amount": 99.99,
  "currency": "USD",
  "processed_at": "2024-01-02T15:04:05Z",
  "idempotency_key": "payment-unique-key-123"
}
```

### GET /api/v1/payments
List all processed payments (for demo purposes)

### GET /health
Health check endpoint

## Idempotency Behavior

### 1. First Request
- Processes payment normally
- Stores response in idempotency cache
- Returns 201 Created

### 2. Duplicate Request (Same Idempotency Key)
- Returns cached response immediately
- No payment processing occurs
- Returns 200 OK with `X-Idempotent-Replay: true` header

### 3. Concurrent Requests
- First request processes normally
- Concurrent duplicates receive 409 Conflict
- Clients should retry after a delay

## Production Considerations

### 1. Use DynamoDB for Distributed Systems

Replace the memory store with DynamoDB:

```go
// Production setup
cfg, _ := config.LoadDefaultConfig(context.TODO())
dynamoClient := dynamodb.NewFromConfig(cfg)

store := middleware.NewDynamoDBIdempotencyStore(dynamoClient, 
    middleware.DynamoDBStoreConfig{
        TableName: "mockery-idempotency-keys",
    })
```

### 2. Idempotency Key Generation

Good patterns for payment systems:
```javascript
// Client-side JavaScript
function generateIdempotencyKey(userId, action) {
    // Option 1: UUID for each attempt
    return `${userId}-${action}-${crypto.randomUUID()}`;
    
    // Option 2: Deterministic for specific operations
    return `${userId}-subscription-renewal-${billingPeriod}`;
    
    // Option 3: User action based
    return `${userId}-checkout-${sessionId}-submit`;
}
```

### 3. Monitoring

Add metrics for production:
```go
OnDuplicate: func(ctx *lift.Context, record *middleware.IdempotencyRecord) {
    // Increment metrics
    idempotencyHits.Inc()
    
    // Log for analysis
    logger.Info("Duplicate payment prevented", 
        "customer_id", ctx.Get("customer_id"),
        "original_time", record.CreatedAt,
        "amount", record.Response,
    )
}
```

### 4. Error Handling

The middleware caches both successful AND failed responses:
- Prevents repeated charges for declined cards
- Avoids overwhelming payment gateways
- Provides consistent error responses

## Benefits for Mockery

1. **Zero Double Charges**: Network retries are safe
2. **Simplified Code**: No idempotency logic in handlers
3. **Better Testing**: Idempotency is handled by middleware
4. **Consistent Behavior**: All payment endpoints are idempotent
5. **Production Ready**: Battle-tested patterns

## Testing Idempotency

```go
func TestPaymentIdempotency(t *testing.T) {
    // Create app with idempotency
    app := createMockeryApp()
    
    // Process payment
    req := createPaymentRequest("test-key-123")
    resp1 := processRequest(app, req)
    
    // Retry with same key
    resp2 := processRequest(app, req)
    
    // Verify same response
    assert.Equal(t, resp1.TransactionID, resp2.TransactionID)
    assert.Equal(t, "true", resp2.Headers["X-Idempotent-Replay"])
}
```

## Migration Path

1. **Week 1**: Deploy with new header support
2. **Week 2**: Update clients to send idempotency keys
3. **Week 3**: Monitor and verify behavior
4. **Week 4**: Remove old idempotency code

## Questions?

See the [comprehensive guide](../../pkg/middleware/IDEMPOTENCY_GUIDE.md) for more details.