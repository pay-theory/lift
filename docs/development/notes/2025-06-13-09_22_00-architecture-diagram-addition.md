# Architecture Diagram Addition to README.md

## Date: 2025-06-13

## Issue Identified
The user pointed out that the README.md file referenced "The diagram above" but didn't actually contain a diagram. This was found by checking the [GitHub repository](https://github.com/pay-theory/lift/blob/premain/docs/README.md).

## Solution Implemented
Added a comprehensive Mermaid diagram to illustrate the Lift framework architecture.

### Diagram Components

1. **Lambda Event Sources**
   - API Gateway V1/V2
   - WebSocket
   - SQS
   - S3
   - EventBridge
   - DynamoDB Streams
   - Scheduled/CloudWatch

2. **Lift Framework Pipeline**
   - Event Detection
   - Event Adapter
   - Router
   - Middleware Chain
   - Handler
   - Response Builder

3. **Middleware Stack**
   - Authentication
   - Rate Limiting
   - Logging
   - Metrics
   - Error Recovery
   - Multi-Tenant

4. **Handler Types**
   - Basic Handler
   - Typed Handler
   - WebSocket Handler

### Visual Flow
The diagram shows:
- Solid arrows (→) for the main event flow
- Dotted arrows (-.→) for optional middleware and handler connections
- Clear separation of concerns with subgraphs
- End-to-end flow from event sources to Lambda response

## Technical Details
- Used Mermaid syntax with `graph LR` (Left to Right) layout
- Embedded directly in markdown using triple backticks with `mermaid` language identifier
- Compatible with GitHub's markdown rendering and most documentation viewers

## Impact
This addition provides visual clarity to the architecture description and helps developers quickly understand how Lift processes different Lambda event sources through a unified pipeline. 