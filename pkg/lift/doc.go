// Package lift provides a Lambda‑native framework for building serverless
// applications in Go. It offers a unified Context that standardizes request
// parsing, response writing, authentication claims, logging, and metrics across
// HTTP and non‑HTTP event sources. Lift includes a simple router, an event
// adapter registry (API Gateway v1/v2, SQS, S3, EventBridge, WebSocket), typed
// handler helpers, structured errors, and production‑ready middleware.
//
// Typical usage
//
//	app := lift.New()
//	app.Use(middleware.RequestID())
//	app.Use(middleware.Logger())
//	app.Use(middleware.Recover())
//
//	// HTTP route with typed handler
//	app.POST("/users", lift.SimpleHandler(CreateUser))
//
//	// SQS/EventBridge handlers can be added via app.Handle("SQS", pattern, handler)
//
//	// In Lambda: lambda.Start(app.HandleRequest)
//
// The design is intentionally opinionated to provide consistent, production‑safe
// patterns and to reduce boilerplate when building event‑driven backends.
package lift
