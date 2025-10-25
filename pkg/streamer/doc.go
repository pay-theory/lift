// Package streamer provides WebSocket connection management for AWS API Gateway.
//
// This package offers a clean, unified interface for managing WebSocket connections
// through AWS API Gateway Management API, with robust error handling and connection
// lifecycle management.
//
// # Basic Usage
//
// Create a client using NewClient:
//
//	client, err := streamer.NewClient(ctx, streamer.ClientConfig{
//		Endpoint: "https://abc123.execute-api.us-west-2.amazonaws.com/production",
//		Region:   "us-west-2", // Optional: will be extracted from endpoint if omitted
//	})
//	if err != nil {
//		log.Fatal(err)
//	}
//
// Send messages to connections:
//
//	err = client.PostToConnection(ctx, connectionID, data)
//	if err != nil {
//		if errors.Is(err, streamer.ErrConnectionGone) {
//			// Handle disconnected connection
//		}
//	}
//
// Get connection information:
//
//	info, err := client.GetConnection(ctx, connectionID)
//	if err != nil {
//		log.Fatal(err)
//	}
//	fmt.Printf("Active: %v, Age: %v\n", info.IsActive(), info.Age())
//
// # Error Handling
//
// The package provides structured error types that match AWS API Gateway responses:
//
//   - GoneError (410): Connection no longer exists
//   - ForbiddenError (403): Operation not permitted
//   - PayloadTooLargeError (413): Message exceeds size limit
//   - ThrottlingError (429): Rate limit exceeded
//   - InternalServerError (500): AWS service error
//
// All errors implement the APIError interface with HTTP status codes and retry information.
//
// # Integration with Lift
//
// This package integrates seamlessly with Lift's WebSocket context:
//
//	func handler(ctx *lift.Context) error {
//		wsCtx, err := ctx.AsWebSocket()
//		if err != nil {
//			return err
//		}
//
//		// Create streamer client from WebSocket context
//		client, err := streamer.NewClient(ctx.Context, streamer.ClientConfig{
//			Endpoint: wsCtx.ManagementEndpoint(),
//			Region:   wsCtx.GetRegion(),
//		})
//		if err != nil {
//			return err
//		}
//
//		return client.PostToConnection(ctx.Context, connectionID, data)
//	}
//
// # Region Resolution
//
// The client automatically resolves the AWS region from multiple sources:
//
//  1. Explicit cfg.Region parameter
//  2. Region extracted from endpoint URL (e.g., "us-west-2" from execute-api URL)
//  3. AWS_REGION environment variable
//  4. AWS_DEFAULT_REGION environment variable
//  5. Default fallback to "us-east-1"
//
// This ensures correct SigV4 request signing even when the region is not explicitly provided.
package streamer
