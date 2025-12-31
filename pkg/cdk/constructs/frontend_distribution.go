package constructs

import (
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"github.com/pay-theory/lift/pkg/naming"
)

// FrontendDistributionProps defines properties for a multi-origin "frontend + API" distribution.
type FrontendDistributionProps struct {
	// Required: hosted zone authoritative for DomainName.
	HostedZone awsroute53.IHostedZone
	// Optional: custom domain certificate. If omitted, Lift creates a DNS-validated certificate in us-east-1.
	Certificate awscertificatemanager.ICertificate
	// Optional: override response headers policy for static content.
	ResponseHeadersPolicy awscloudfront.IResponseHeadersPolicy
	// Optional: cache policy for API behaviors.
	// Note: Authorization must be forwarded via CachePolicy (not OriginRequestPolicy).
	ApiCachePolicy awscloudfront.ICachePolicy
	// Optional: origin request policy for API behaviors (default: none).
	ApiOriginRequestPolicy awscloudfront.IOriginRequestPolicy

	// Required: apex/canonical domain (e.g., "example.com").
	DomainName *string

	// Required: API origin host (e.g., "api.example.com" or "*.execute-api.*.amazonaws.com").
	ApiOriginDomainName *string

	// Optional: stable naming inputs for deterministic bucket names.
	AppName    *string
	Stage      *string
	Partner    *string
	BucketName *string

	// Optional: bucket configuration.
	AutoDeleteObjects *bool
	Versioned         *bool

	// Optional: additional SANs to include when Lift creates a certificate.
	SubjectAlternativeNames *[]*string

	// Optional: enable www.<domain> redirect to apex (default true).
	EnableWWWRedirect *bool
	WWWDomainName     *string

	// Optional: treat site as SPA by serving /index.html for 403/404 (default false).
	SinglePageApp *bool

	// Optional: WAFv2 Web ACL ARN (global scope) to attach to the distribution.
	WebAclId *string

	// Optional: distribution tuning.
	EnableIpv6 *bool

	// Optional: cache path patterns that should receive long-lived "hashed asset" caching.
	HashedAssetPathPatterns *[]*string

	// Optional: path patterns routed to the API origin (default: ["api/*", "graphql", ".well-known/*"]).
	ApiPathPatterns *[]*string

	// Optional: tags applied to created resources.
	Tags *map[string]*string

	// Optional: bucket configuration.
	RemovalPolicy awscdk.RemovalPolicy

	// Optional: distribution tuning.
	PriceClass  awscloudfront.PriceClass
	HttpVersion awscloudfront.HttpVersion
}

// FrontendDistribution creates an S3-backed static site with API proxy behaviors.
type FrontendDistribution struct {
	constructs.Construct

	Bucket       awss3.Bucket
	Distribution awscloudfront.Distribution
	Certificate  awscertificatemanager.ICertificate

	WWWRedirect *HostRedirect
}

// NewFrontendDistribution creates a CloudFront distribution with a static S3 origin and API proxy behaviors.
func NewFrontendDistribution(scope constructs.Construct, id *string, props *FrontendDistributionProps) *FrontendDistribution {
	construct := constructs.NewConstruct(scope, id)
	props = normalizeFrontendDistributionProps(props)

	nameCtx, hasNameCtx := cloudfrontNamingContext(props.AppName, props.Stage, props.Partner)

	frontend := &FrontendDistribution{Construct: construct}

	bucketName := resolveS3BucketName(construct, id, props.BucketName, nameCtx, hasNameCtx, "frontend")
	frontend.Bucket = awss3.NewBucket(construct, jsii.String("Bucket"), &awss3.BucketProps{
		BucketName:        bucketName,
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EnforceSSL:        jsii.Bool(true),
		ObjectOwnership:   awss3.ObjectOwnership_BUCKET_OWNER_ENFORCED,
		RemovalPolicy:     props.RemovalPolicy,
		AutoDeleteObjects: props.AutoDeleteObjects,
		Versioned:         props.Versioned,
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
	})

	applyStandardConstructTags(frontend.Bucket, props.Tags, nameCtx, hasNameCtx, "FrontendDistribution")

	frontend.Certificate = resolveFrontendDistributionCertificate(construct, props)

	staticOrigin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(frontend.Bucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{})

	htmlCache := newStaticSiteHTMLCachePolicy(construct, nameCtx, hasNameCtx)
	assetCache := newStaticSiteAssetCachePolicy(construct, nameCtx, hasNameCtx)
	headersPolicy := resolveFrontendDistributionHeadersPolicy(construct, props)

	defaultBehavior := &awscloudfront.BehaviorOptions{
		Origin:                staticOrigin,
		ViewerProtocolPolicy:  awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		CachePolicy:           htmlCache,
		ResponseHeadersPolicy: headersPolicy,
		Compress:              jsii.Bool(true),
	}

	errorResponses := errorResponsesForSinglePageApp(props.SinglePageApp)

	frontend.Distribution = awscloudfront.NewDistribution(construct, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		DefaultBehavior:   defaultBehavior,
		DomainNames:       &[]*string{props.DomainName},
		Certificate:       frontend.Certificate,
		DefaultRootObject: jsii.String("index.html"),
		EnableIpv6:        props.EnableIpv6,
		HttpVersion:       props.HttpVersion,
		PriceClass:        props.PriceClass,
		WebAclId:          props.WebAclId,
		ErrorResponses:    &errorResponses,
	})

	applyStandardConstructTags(frontend.Distribution, props.Tags, nameCtx, hasNameCtx, "FrontendDistribution")
	addHashedAssetBehaviors(frontend.Distribution, staticOrigin, props.HashedAssetPathPatterns, assetCache, headersPolicy)
	addFrontendDistributionAPIBehaviors(construct, frontend.Distribution, props, nameCtx, hasNameCtx)
	addCloudFrontAliasRecords(construct, props.HostedZone, props.DomainName, frontend.Distribution, props.EnableIpv6)
	frontend.WWWRedirect = maybeAddFrontendWWWRedirect(construct, props, frontend.Certificate)

	return frontend
}

func normalizeFrontendDistributionProps(props *FrontendDistributionProps) *FrontendDistributionProps {
	if props == nil {
		props = &FrontendDistributionProps{}
	}
	if props.DomainName == nil || strings.TrimSpace(*props.DomainName) == "" {
		panic("FrontendDistribution requires DomainName")
	}
	if props.HostedZone == nil {
		panic("FrontendDistribution requires HostedZone")
	}
	if props.ApiOriginDomainName == nil || strings.TrimSpace(*props.ApiOriginDomainName) == "" {
		panic("FrontendDistribution requires ApiOriginDomainName")
	}

	ensureBool(&props.EnableWWWRedirect, true)
	ensureBool(&props.SinglePageApp, false)
	ensureBool(&props.EnableIpv6, true)
	ensureHttpVersion(&props.HttpVersion, awscloudfront.HttpVersion_HTTP2)
	ensurePriceClass(&props.PriceClass, awscloudfront.PriceClass_PRICE_CLASS_ALL)
	ensureRemovalPolicy(&props.RemovalPolicy, awscdk.RemovalPolicy_RETAIN)
	ensureBool(&props.Versioned, false)
	defaultAutoDeleteObjectsIfDestroy(props.RemovalPolicy, &props.AutoDeleteObjects)

	return props
}

func resolveFrontendDistributionCertificate(scope constructs.Construct, props *FrontendDistributionProps) awscertificatemanager.ICertificate {
	if props.Certificate != nil {
		return props.Certificate
	}

	sans := []*string{}
	if props.SubjectAlternativeNames != nil {
		sans = append(sans, (*props.SubjectAlternativeNames)...)
	}
	if props.EnableWWWRedirect != nil && *props.EnableWWWRedirect {
		www := resolveWWWDomainName(props.DomainName, props.WWWDomainName)
		sans = append(sans, www)
		props.WWWDomainName = www
	}

	return ensureCloudFrontCertificate(scope, nil, props.DomainName, props.HostedZone, sans)
}

func resolveFrontendDistributionHeadersPolicy(scope constructs.Construct, props *FrontendDistributionProps) awscloudfront.IResponseHeadersPolicy {
	if props.ResponseHeadersPolicy != nil {
		return props.ResponseHeadersPolicy
	}
	return newStaticSiteResponseHeadersPolicy(scope, props.DomainName)
}

func addFrontendDistributionAPIBehaviors(scope constructs.Construct, distribution awscloudfront.Distribution, props *FrontendDistributionProps, nameCtx naming.Context, hasNameCtx bool) {
	apiPatterns := props.ApiPathPatterns
	if apiPatterns == nil {
		apiPatterns = &[]*string{
			jsii.String("api/*"),
			jsii.String("graphql"),
			jsii.String(".well-known/*"),
		}
	}

	apiOrigin := awscloudfrontorigins.NewHttpOrigin(props.ApiOriginDomainName, &awscloudfrontorigins.HttpOriginProps{
		ProtocolPolicy: awscloudfront.OriginProtocolPolicy_HTTPS_ONLY,
	})

	apiCachePolicy := props.ApiCachePolicy
	if apiCachePolicy == nil {
		apiCachePolicy = defaultAPICachePolicy(scope, nameCtx, hasNameCtx)
	}

	apiOriginRequestPolicy := props.ApiOriginRequestPolicy
	if apiOriginRequestPolicy == nil {
		apiOriginRequestPolicy = defaultAPIOriginRequestPolicy(scope, nameCtx, hasNameCtx)
	}

	for _, pattern := range *apiPatterns {
		if pattern == nil || strings.TrimSpace(*pattern) == "" {
			continue
		}
		distribution.AddBehavior(pattern, apiOrigin, &awscloudfront.AddBehaviorOptions{
			AllowedMethods:       awscloudfront.AllowedMethods_ALLOW_ALL(),
			CachedMethods:        awscloudfront.CachedMethods_CACHE_GET_HEAD_OPTIONS(),
			CachePolicy:          apiCachePolicy,
			OriginRequestPolicy:  apiOriginRequestPolicy,
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			Compress:             jsii.Bool(true),
		})
	}
}

func maybeAddFrontendWWWRedirect(scope constructs.Construct, props *FrontendDistributionProps, cert awscertificatemanager.ICertificate) *HostRedirect {
	if props.EnableWWWRedirect == nil || !*props.EnableWWWRedirect {
		return nil
	}

	www := resolveWWWDomainName(props.DomainName, props.WWWDomainName)
	return NewHostRedirect(scope, jsii.String("WWWRedirect"), &HostRedirectProps{
		FromDomainName: www,
		ToDomainName:   props.DomainName,
		HostedZone:     props.HostedZone,
		Certificate:    cert,
		WebAclId:       props.WebAclId,
		EnableIpv6:     props.EnableIpv6,
		Tags:           props.Tags,
	})
}
