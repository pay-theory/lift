package constructs

import (
	"fmt"
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// PathRoutedFrontendDistributionProps defines properties for a single-domain distribution that routes
// API traffic to an origin by default while serving one or more SPAs under path prefixes.
type PathRoutedFrontendDistributionProps struct {
	// Required: hosted zone authoritative for DomainName.
	HostedZone awsroute53.IHostedZone
	// Optional: custom domain certificate. If omitted, Lift creates a DNS-validated certificate in us-east-1.
	Certificate awscertificatemanager.ICertificate

	// Optional: cache policy for API behavior (default: Lift API cache policy with TTL 0).
	ApiCachePolicy awscloudfront.ICachePolicy
	// Optional: origin request policy for API behavior (default: none).
	ApiOriginRequestPolicy awscloudfront.IOriginRequestPolicy

	// Optional: override response headers policy for static content.
	StaticResponseHeadersPolicy awscloudfront.IResponseHeadersPolicy

	// Required: apex/canonical domain (e.g., "dev.example.com").
	DomainName *string
	// Required: API origin host (e.g., "api.dev.example.com" or "*.execute-api.*.amazonaws.com").
	ApiOriginDomainName *string

	// Required: S3 bucket name for the client SPA served under ClientPathPrefix.
	ClientBucketName *string
	// Required: S3 bucket name for the auth SPA served under AuthPathPrefix.
	AuthBucketName *string

	// Optional: path prefix for client SPA (default: "l").
	ClientPathPrefix *string
	// Optional: path prefix for auth SPA (default: "auth").
	AuthPathPrefix *string
	// Optional: API path pattern that must bypass the auth SPA (default: "<authPrefix>/wallet/*").
	AuthWalletApiPathPattern *string
	// Optional: treat client UI as an SPA by rewriting extensionless routes to /index.html (default true).
	ClientSinglePageApp *bool
	// Optional: treat auth UI as an SPA by rewriting extensionless routes to /index.html (default true).
	// If false, extensionless routes rewrite to "<path>/index.html" to support multi-page static outputs.
	AuthSinglePageApp *bool

	// Optional: stable naming inputs for deterministic resource naming.
	AppName *string
	Stage   *string
	Partner *string

	// Optional: tags applied to created resources.
	Tags *map[string]*string

	// Optional: bucket configuration.
	AutoDeleteObjects *bool
	Versioned         *bool
	RemovalPolicy     awscdk.RemovalPolicy

	// Optional: WAFv2 Web ACL ARN (global scope) to attach to the distribution.
	WebAclId *string

	// Optional: distribution tuning.
	EnableIpv6  *bool
	PriceClass  awscloudfront.PriceClass
	HttpVersion awscloudfront.HttpVersion
}

// PathRoutedFrontendDistribution creates a single CloudFront distribution for a stage domain that:
// - routes all unmatched requests to the API origin (default behavior)
// - serves a client SPA under /<clientPrefix>/*
// - serves an auth SPA under /<authPrefix>/*
// - reserves /<authPrefix>/wallet/* for the API origin
type PathRoutedFrontendDistribution struct {
	constructs.Construct

	ClientBucket awss3.Bucket
	AuthBucket   awss3.Bucket

	Distribution awscloudfront.Distribution
	Certificate  awscertificatemanager.ICertificate

	RewriteFunction awscloudfront.Function
}

func NewPathRoutedFrontendDistribution(scope constructs.Construct, id *string, props *PathRoutedFrontendDistributionProps) *PathRoutedFrontendDistribution {
	construct := constructs.NewConstruct(scope, id)
	props = normalizePathRoutedFrontendDistributionProps(props)

	nameCtx, hasNameCtx := cloudfrontNamingContext(props.AppName, props.Stage, props.Partner)

	dist := &PathRoutedFrontendDistribution{Construct: construct}

	dist.ClientBucket = awss3.NewBucket(construct, jsii.String("ClientBucket"), &awss3.BucketProps{
		BucketName:        resolveS3BucketName(construct, jsii.String("ClientBucket"), props.ClientBucketName, nameCtx, hasNameCtx, "client"),
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EnforceSSL:        jsii.Bool(true),
		ObjectOwnership:   awss3.ObjectOwnership_BUCKET_OWNER_ENFORCED,
		RemovalPolicy:     props.RemovalPolicy,
		AutoDeleteObjects: props.AutoDeleteObjects,
		Versioned:         props.Versioned,
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
	})
	applyStandardConstructTags(dist.ClientBucket, props.Tags, nameCtx, hasNameCtx, "PathRoutedFrontendDistribution")

	dist.AuthBucket = awss3.NewBucket(construct, jsii.String("AuthBucket"), &awss3.BucketProps{
		BucketName:        resolveS3BucketName(construct, jsii.String("AuthBucket"), props.AuthBucketName, nameCtx, hasNameCtx, "auth-ui"),
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EnforceSSL:        jsii.Bool(true),
		ObjectOwnership:   awss3.ObjectOwnership_BUCKET_OWNER_ENFORCED,
		RemovalPolicy:     props.RemovalPolicy,
		AutoDeleteObjects: props.AutoDeleteObjects,
		Versioned:         props.Versioned,
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
	})
	applyStandardConstructTags(dist.AuthBucket, props.Tags, nameCtx, hasNameCtx, "PathRoutedFrontendDistribution")

	dist.Certificate = resolvePathRoutedFrontendDistributionCertificate(construct, props)

	apiOrigin := awscloudfrontorigins.NewHttpOrigin(props.ApiOriginDomainName, &awscloudfrontorigins.HttpOriginProps{
		ProtocolPolicy: awscloudfront.OriginProtocolPolicy_HTTPS_ONLY,
	})
	clientOrigin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(dist.ClientBucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{})
	authOrigin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(dist.AuthBucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{})

	apiCachePolicy := props.ApiCachePolicy
	if apiCachePolicy == nil {
		apiCachePolicy = defaultAPICachePolicy(construct, nameCtx, hasNameCtx)
	}

	apiOriginRequestPolicy := props.ApiOriginRequestPolicy
	if apiOriginRequestPolicy == nil {
		apiOriginRequestPolicy = defaultAPIOriginRequestPolicy(construct, nameCtx, hasNameCtx)
	}

	staticCache := newStaticSiteHTMLCachePolicy(construct, nameCtx, hasNameCtx)
	staticHeaders := resolvePathRoutedFrontendDistributionStaticHeadersPolicy(construct, props)

	dist.RewriteFunction = newPathPrefixSPAFallbackFunction(construct, props)

	dist.Distribution = awscloudfront.NewDistribution(construct, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		DomainNames: &[]*string{props.DomainName},
		Certificate: dist.Certificate,
		EnableIpv6:  props.EnableIpv6,
		HttpVersion: props.HttpVersion,
		PriceClass:  props.PriceClass,
		WebAclId:    props.WebAclId,
		DefaultBehavior: &awscloudfront.BehaviorOptions{
			Origin:               apiOrigin,
			AllowedMethods:       awscloudfront.AllowedMethods_ALLOW_ALL(),
			CachedMethods:        awscloudfront.CachedMethods_CACHE_GET_HEAD_OPTIONS(),
			CachePolicy:          apiCachePolicy,
			OriginRequestPolicy:  apiOriginRequestPolicy,
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			Compress:             jsii.Bool(true),
		},
	})

	applyStandardConstructTags(dist.Distribution, props.Tags, nameCtx, hasNameCtx, "PathRoutedFrontendDistribution")
	addCloudFrontAliasRecords(construct, props.HostedZone, props.DomainName, dist.Distribution, props.EnableIpv6)

	authPrefix, clientPrefix, walletPattern := resolvePathRoutedFrontendDistributionPrefixes(props)

	// Ensure API-owned wallet endpoints under /<authPrefix>/wallet/* bypass the auth SPA behavior.
	dist.Distribution.AddBehavior(jsii.String(walletPattern), apiOrigin, &awscloudfront.AddBehaviorOptions{
		AllowedMethods:       awscloudfront.AllowedMethods_ALLOW_ALL(),
		CachedMethods:        awscloudfront.CachedMethods_CACHE_GET_HEAD_OPTIONS(),
		CachePolicy:          apiCachePolicy,
		OriginRequestPolicy:  apiOriginRequestPolicy,
		ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		Compress:             jsii.Bool(true),
	})

	authBehavior := &awscloudfront.AddBehaviorOptions{
		AllowedMethods:        awscloudfront.AllowedMethods_ALLOW_GET_HEAD_OPTIONS(),
		CachedMethods:         awscloudfront.CachedMethods_CACHE_GET_HEAD_OPTIONS(),
		CachePolicy:           staticCache,
		ResponseHeadersPolicy: staticHeaders,
		ViewerProtocolPolicy:  awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		Compress:              jsii.Bool(true),
		FunctionAssociations: &[]*awscloudfront.FunctionAssociation{
			{
				Function:  dist.RewriteFunction,
				EventType: awscloudfront.FunctionEventType_VIEWER_REQUEST,
			},
		},
	}

	clientBehavior := &awscloudfront.AddBehaviorOptions{
		AllowedMethods:        awscloudfront.AllowedMethods_ALLOW_GET_HEAD_OPTIONS(),
		CachedMethods:         awscloudfront.CachedMethods_CACHE_GET_HEAD_OPTIONS(),
		CachePolicy:           staticCache,
		ResponseHeadersPolicy: staticHeaders,
		ViewerProtocolPolicy:  awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		Compress:              jsii.Bool(true),
		FunctionAssociations: &[]*awscloudfront.FunctionAssociation{
			{
				Function:  dist.RewriteFunction,
				EventType: awscloudfront.FunctionEventType_VIEWER_REQUEST,
			},
		},
	}

	dist.Distribution.AddBehavior(jsii.String(authPrefix), authOrigin, authBehavior)
	dist.Distribution.AddBehavior(jsii.String(authPrefix+"/*"), authOrigin, authBehavior)
	dist.Distribution.AddBehavior(jsii.String(clientPrefix), clientOrigin, clientBehavior)
	dist.Distribution.AddBehavior(jsii.String(clientPrefix+"/*"), clientOrigin, clientBehavior)

	return dist
}

func normalizePathRoutedFrontendDistributionProps(props *PathRoutedFrontendDistributionProps) *PathRoutedFrontendDistributionProps {
	if props == nil {
		props = &PathRoutedFrontendDistributionProps{}
	}
	if props.DomainName == nil || strings.TrimSpace(*props.DomainName) == "" {
		panic("PathRoutedFrontendDistribution requires DomainName")
	}
	if props.HostedZone == nil {
		panic("PathRoutedFrontendDistribution requires HostedZone")
	}
	if props.ApiOriginDomainName == nil || strings.TrimSpace(*props.ApiOriginDomainName) == "" {
		panic("PathRoutedFrontendDistribution requires ApiOriginDomainName")
	}
	if props.ClientBucketName == nil || strings.TrimSpace(*props.ClientBucketName) == "" {
		panic("PathRoutedFrontendDistribution requires ClientBucketName")
	}
	if props.AuthBucketName == nil || strings.TrimSpace(*props.AuthBucketName) == "" {
		panic("PathRoutedFrontendDistribution requires AuthBucketName")
	}

	ensureBool(&props.EnableIpv6, true)
	ensureBool(&props.ClientSinglePageApp, true)
	ensureBool(&props.AuthSinglePageApp, true)
	ensureHttpVersion(&props.HttpVersion, awscloudfront.HttpVersion_HTTP2)
	ensurePriceClass(&props.PriceClass, awscloudfront.PriceClass_PRICE_CLASS_ALL)
	ensureRemovalPolicy(&props.RemovalPolicy, awscdk.RemovalPolicy_RETAIN)
	ensureBool(&props.Versioned, false)
	defaultAutoDeleteObjectsIfDestroy(props.RemovalPolicy, &props.AutoDeleteObjects)

	return props
}

func resolvePathRoutedFrontendDistributionCertificate(scope constructs.Construct, props *PathRoutedFrontendDistributionProps) awscertificatemanager.ICertificate {
	if props.Certificate != nil {
		return props.Certificate
	}
	return ensureCloudFrontCertificate(scope, nil, props.DomainName, props.HostedZone, nil)
}

func resolvePathRoutedFrontendDistributionStaticHeadersPolicy(scope constructs.Construct, props *PathRoutedFrontendDistributionProps) awscloudfront.IResponseHeadersPolicy {
	if props.StaticResponseHeadersPolicy != nil {
		return props.StaticResponseHeadersPolicy
	}
	return newStaticSiteResponseHeadersPolicy(scope, props.DomainName)
}

func resolvePathRoutedFrontendDistributionPrefixes(props *PathRoutedFrontendDistributionProps) (authPrefix string, clientPrefix string, walletPattern string) {
	authPrefix = normalizePathPrefix(props.AuthPathPrefix, "auth")
	clientPrefix = normalizePathPrefix(props.ClientPathPrefix, "l")

	if props.AuthWalletApiPathPattern != nil && strings.TrimSpace(*props.AuthWalletApiPathPattern) != "" {
		walletPattern = normalizePathPattern(*props.AuthWalletApiPathPattern)
	} else {
		walletPattern = authPrefix + "/wallet/*"
	}

	return authPrefix, clientPrefix, walletPattern
}

func normalizePathPrefix(prefix *string, def string) string {
	if prefix == nil {
		return def
	}
	out := strings.TrimSpace(*prefix)
	out = strings.Trim(out, "/")
	if out == "" {
		return def
	}
	return out
}

func normalizePathPattern(pattern string) string {
	out := strings.TrimSpace(pattern)
	out = strings.TrimPrefix(out, "/")
	out = strings.TrimSuffix(out, "/")
	return out
}

func newPathPrefixSPAFallbackFunction(scope constructs.Construct, props *PathRoutedFrontendDistributionProps) awscloudfront.Function {
	authPrefix, clientPrefix, _ := resolvePathRoutedFrontendDistributionPrefixes(props)
	authSPA := props.AuthSinglePageApp != nil && *props.AuthSinglePageApp
	clientSPA := props.ClientSinglePageApp != nil && *props.ClientSinglePageApp

	code := fmt.Sprintf(`function handler(event) {
  var request = event.request;
  var uri = request.uri || "/";

  function isFilePath(path) {
    var lastSlash = path.lastIndexOf("/");
    var lastDot = path.lastIndexOf(".");
    return lastDot > lastSlash;
  }

  function rewrite(prefix, isSPA) {
    var p = "/" + prefix;
    if (uri === p || uri === p + "/") {
      request.uri = "/index.html";
      return request;
    }
    if (uri.indexOf(p + "/") !== 0) {
      return null;
    }

    var out = uri.substring(p.length);
    if (out === "" || out === "/") {
      request.uri = "/index.html";
      return request;
    }
    if (out.endsWith("/")) {
      request.uri = isSPA ? "/index.html" : (out + "index.html");
      return request;
    }
    if (!isFilePath(out)) {
      request.uri = isSPA ? "/index.html" : (out + "/index.html");
      return request;
    }
    request.uri = out;
    return request;
  }

  return rewrite("%s", %t) || rewrite("%s", %t) || request;
}`, authPrefix, authSPA, clientPrefix, clientSPA)

	return awscloudfront.NewFunction(scope, jsii.String("PathPrefixSPAFallbackFunction"), &awscloudfront.FunctionProps{
		Code:    awscloudfront.FunctionCode_FromInline(jsii.String(code)),
		Runtime: awscloudfront.FunctionRuntime_JS_2_0(),
	})
}
