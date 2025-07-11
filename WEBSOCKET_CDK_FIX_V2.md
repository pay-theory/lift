# WebSocket CDK Fix v1.0.51

## Issue
WebSocket CDK deployments fail with CloudFormation errors due to excessively long Lambda permission resource names (up to 90 characters).

## Root Cause
AWS CDK automatically generates resource names by concatenating:
- Parent construct ID (e.g., `PennyWebSocketAPI`)
- Nested resource IDs (e.g., `WebSocketApi`, `connectRoute`, `connectIntegration`)
- Resource type (e.g., `Permission`)
- Random hash for uniqueness

This creates names like:
`PennyWebSocketAPIWebSocketApiconnectRouteconnectIntPermission6DDC7D18` (69 chars)

## Solution (v1.0.51)
Minimized all intermediate resource IDs:

1. **API Resource**: `WebSocketApi` → `Api` 
2. **Function IDs**: `ConnectFunction` → `C`, `DisconnectFunction` → `D`, `DefaultFunction` → `X`
3. **Integration IDs**: `connectIntegration` → `C`, `disconnectIntegration` → `D`, `defaultIntegration` → `X`
4. **Connection Table**: `ConnectionTable` → `T`

## Results
Maximum permission name lengths:
- `WS` construct: 39 chars ✅
- `WebSocket` construct: 46 chars ✅ 
- `MyWebSocketAPI` construct: 51 chars ✅
- `PennyWebSocketAPI` construct: 54 chars ✅

All well under CloudFormation limits!

## Recommendations for Users
Keep your WebSocket construct IDs short:
- ✅ Good: `WS`, `WSApi`, `WebSocket`
- ⚠️ OK: `MyWebSocketAPI` 
- ❌ Avoid: `MyCompanyWebSocketAPI`, `ProductionWebSocketAPIGateway`

## Migration
No code changes required - just update to v1.0.51:
```bash
go get github.com/pay-theory/lift@v1.0.51
```