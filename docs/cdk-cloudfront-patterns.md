# CloudFront Patterns (Static Sites + Media CDN)

Lift includes first-class CDK constructs for CloudFront-backed static frontends and media CDNs with deterministic naming, OAC-backed private S3 origins, and opinionated defaults.

## Naming Conventions

Lift follows Pay Theory’s stage convention:

- `lab` (dev)
- `study` (sandbox)
- `live` (prod)

All constructs accept `AppName`, `Stage`, and optional `Partner` to build stable names:

- Base: `<repo>-<partner>-<stage>` or `<repo>-<stage>`
- Resource names: `<repo>-<partner>-<resource>-<stage>`

S3 buckets require DNS-safe lowercase names; Lift automatically sanitizes internally generated names.

## Static Site (Apex Canonical + www Redirect)

Use `constructs.NewStaticSite` to create:

- Private S3 bucket (OAC-only)
- CloudFront distribution with security headers + safe caching defaults
- ACM certificate in `us-east-1`
- Route53 A/AAAA alias for the apex domain
- Optional `www.<domain> -> <domain>` redirect (CloudFront Function, `308`)

```go
zone := awsroute53.HostedZone_FromHostedZoneAttributes(stack, jsii.String("Zone"), &awsroute53.HostedZoneAttributes{
  HostedZoneId: jsii.String("Z123"),
  ZoneName:     jsii.String("example.com"),
})

site := constructs.NewStaticSite(stack, jsii.String("Site"), &constructs.StaticSiteProps{
  DomainName: jsii.String("example.com"),
  HostedZone: zone,

  // Deterministic naming (recommended)
  AppName:  jsii.String("lesser"),
  Stage:    jsii.String("live"),
  Partner:  jsii.String("tenant-a"), // optional

  // Optional DX
  SinglePageApp:     jsii.Bool(true),  // serve /index.html for 403/404
  EnableWWWRedirect: jsii.Bool(true),  // default true
})
_ = site
```

### Defaults & Guardrails

- S3 origin is private (`BlockPublicAccess=BLOCK_ALL`, `EnforceSSL=true`) and accessed via **OAC**.
- Default behavior caches HTML short-lived; hashed asset path patterns get long-lived caching.
- Default response headers include HSTS, CSP template, referrer policy, and frame protections.
- `www -> apex` redirect is implemented with a **CloudFront Function** (no Lambda@Edge) returning **308**.

### Common Customizations

- Access logs: `EnableAccessLogs`, `AccessLogsBucket`, `AccessLogsPrefix`
- WAF: `WebAclId` (WAFv2 ARN in the CloudFront/global scope)
- Caching: `HashedAssetPathPatterns` (asset behaviors), `ResponseHeadersPolicy` (security headers)
- Distribution tuning: `PriceClass`, `HttpVersion`, `EnableIpv6`

## Single-Host “Frontend Distribution” (Static + API Proxy)

Use `constructs.NewFrontendDistribution` to serve UI from S3 and proxy selected paths to an API origin (no SSR):

- Default origin: private S3 (static)
- API origin: `HttpOrigin` (e.g. `api.example.com` or API Gateway domain)
- Default API path patterns: `api/*`, `graphql`, `.well-known/*`
- API cache policy uses TTL=0 and forwards query strings + allowlisted headers (including `Authorization`)

```go
frontend := constructs.NewFrontendDistribution(stack, jsii.String("Frontend"), &constructs.FrontendDistributionProps{
  DomainName:          jsii.String("example.com"),
  HostedZone:          zone,
  ApiOriginDomainName: jsii.String("api.example.com"),

  AppName: jsii.String("lesser"),
  Stage:   jsii.String("study"),
})
_ = frontend
```

### API Proxy Customizations

- Route selection: `ApiPathPatterns`
- Header/query forwarding: `ApiCachePolicy` (TTL=0, includes `Authorization`), `ApiOriginRequestPolicy` (additional forwarding controls)

## Media CDN (Optional Private Media)

Use `constructs.NewMediaCDN` to create a CloudFront distribution tuned for media:

- Private S3 origin via OAC
- Media cache policy defaults (longer TTLs)
- Optional signed URL/cookie support via Key Groups

```go
media := constructs.NewMediaCDN(stack, jsii.String("Media"), &constructs.MediaCDNProps{
  DomainName: jsii.String("media.example.com"),
  HostedZone: zone,

  AppName: jsii.String("lesser"),
  Stage:   jsii.String("live"),

  EnablePrivateMedia: jsii.Bool(true),
  PublicKeyEncoded:   jsii.String("-----BEGIN PUBLIC KEY-----\n...\n-----END PUBLIC KEY-----"),
  PrivatePathPatterns: &[]*string{jsii.String("private/*")},
})
_ = media
```

### Runtime Signing Helper

Lift provides a runtime helper for signing URLs and cookies:

- `pkg/security/cloudfront_signing.go`
- Uses `SecretsProvider` (e.g. `security.NewAWSSecretsManager`) to load the private key PEM.

```go
secrets, _ := security.NewAWSSecretsManager(ctx, "us-east-1", "")
signer, _ := security.NewCloudFrontSignerFromSecrets(ctx, secrets, keyPairID, "cloudfront/private-key")

signed, _ := signer.SignURL("https://media.example.com/private/video.mp4", time.Now().Add(15*time.Minute))
_ = signed
```

### Private Media Notes

- `EnablePrivateMedia` creates a CloudFront `PublicKey` + `KeyGroup` and applies it to `PrivatePathPatterns` (default: `private/*`).
- The runtime signer is intentionally SecretsProvider-backed so private key material can live in Secrets Manager (or any provider implementing the interface).

## Host Redirect (Generic)

If you need a standalone redirect distribution (not tied to `NewStaticSite`), use `constructs.NewHostRedirect`:

```go
constructs.NewHostRedirect(stack, jsii.String("Redirect"), &constructs.HostRedirectProps{
  FromDomainName: jsii.String("www.example.com"),
  ToDomainName:   jsii.String("example.com"),
  HostedZone:     zone,
})
```

## Current Scope (What Lift Creates Today)

These constructs focus on **safe, repeatable infrastructure** (buckets, distributions, certs, DNS). Lift does not currently ship:

- A static “publish” pipeline (hashed asset upload + minimal invalidations)
- Preview environments / blue-green prefixes
- An EventBus-driven invalidation worker
- Multi-origin “front door” beyond the API proxy behaviors in `NewFrontendDistribution`
