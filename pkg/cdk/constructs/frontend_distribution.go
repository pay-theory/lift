package constructs

import (
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfrontorigins"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// FrontendDistributionProps defines properties for a multi-origin "frontend + API" distribution.
type FrontendDistributionProps struct {
	// Required: apex/canonical domain (e.g., "example.com").
	DomainName *string

	// Required: hosted zone authoritative for DomainName.
	HostedZone awsroute53.IHostedZone

	// Required: API origin host (e.g., "api.example.com" or "*.execute-api.*.amazonaws.com").
	ApiOriginDomainName *string

	// Optional: stable naming inputs for deterministic bucket names.
	AppName    *string
	Stage      *string
	Partner    *string
	BucketName *string

	// Optional: bucket configuration.
	RemovalPolicy     awscdk.RemovalPolicy
	AutoDeleteObjects *bool
	Versioned         *bool

	// Optional: custom domain certificate. If omitted, Lift creates a DNS-validated certificate in us-east-1.
	Certificate awscertificatemanager.ICertificate

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
	PriceClass awscloudfront.PriceClass
	HttpVersion awscloudfront.HttpVersion
	EnableIpv6 *bool

	// Optional: cache path patterns that should receive long-lived "hashed asset" caching.
	HashedAssetPathPatterns *[]*string

	// Optional: override response headers policy for static content.
	ResponseHeadersPolicy awscloudfront.IResponseHeadersPolicy

	// Optional: path patterns routed to the API origin (default: ["api/*", "graphql", ".well-known/*"]).
	ApiPathPatterns *[]*string

	// Optional: cache policy for API behaviors.
	// Note: Authorization must be forwarded via CachePolicy (not OriginRequestPolicy).
	ApiCachePolicy awscloudfront.ICachePolicy

	// Optional: origin request policy for API behaviors (default: none).
	ApiOriginRequestPolicy awscloudfront.IOriginRequestPolicy

	// Optional: tags applied to created resources.
	Tags *map[string]*string
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

	if props.EnableWWWRedirect == nil {
		props.EnableWWWRedirect = jsii.Bool(true)
	}
	if props.SinglePageApp == nil {
		props.SinglePageApp = jsii.Bool(false)
	}
	if props.EnableIpv6 == nil {
		props.EnableIpv6 = jsii.Bool(true)
	}
	if props.HttpVersion == "" {
		props.HttpVersion = awscloudfront.HttpVersion_HTTP2
	}
	if props.PriceClass == "" {
		props.PriceClass = awscloudfront.PriceClass_PRICE_CLASS_ALL
	}
	if props.RemovalPolicy == "" {
		props.RemovalPolicy = awscdk.RemovalPolicy_RETAIN
	}
	if props.Versioned == nil {
		props.Versioned = jsii.Bool(false)
	}
	if props.RemovalPolicy == awscdk.RemovalPolicy_DESTROY && props.AutoDeleteObjects == nil {
		props.AutoDeleteObjects = jsii.Bool(true)
	}

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

	if props.Tags != nil {
		for k, v := range *props.Tags {
			awscdk.Tags_Of(frontend.Bucket).Add(jsii.String(k), v, nil)
		}
	}
	if hasNameCtx {
		awscdk.Tags_Of(frontend.Bucket).Add(jsii.String("Application"), jsii.String(nameCtx.AppName), nil)
		awscdk.Tags_Of(frontend.Bucket).Add(jsii.String("Environment"), jsii.String(nameCtx.Stage), nil)
		if nameCtx.Tenant != "" {
			awscdk.Tags_Of(frontend.Bucket).Add(jsii.String("Partner"), jsii.String(nameCtx.Tenant), nil)
		}
	}
	awscdk.Tags_Of(frontend.Bucket).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(frontend.Bucket).Add(jsii.String("Component"), jsii.String("FrontendDistribution"), nil)

	cert := props.Certificate
	if cert == nil {
		sans := []*string{}
		if props.SubjectAlternativeNames != nil {
			sans = append(sans, (*props.SubjectAlternativeNames)...)
		}
		if *props.EnableWWWRedirect {
			www := props.WWWDomainName
			if www == nil || strings.TrimSpace(*www) == "" {
				www = jsii.String("www." + strings.TrimSuffix(strings.TrimSpace(*props.DomainName), "."))
			}
			sans = append(sans, www)
			props.WWWDomainName = www
		}

		cert = awscertificatemanager.NewDnsValidatedCertificate(construct, jsii.String("Certificate"), &awscertificatemanager.DnsValidatedCertificateProps{
			DomainName:              props.DomainName,
			HostedZone:              props.HostedZone,
			Region:                  jsii.String("us-east-1"),
			SubjectAlternativeNames: &sans,
		})
	}
	frontend.Certificate = cert

	staticOrigin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(frontend.Bucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{})

	htmlCache := newStaticSiteHTMLCachePolicy(construct, nameCtx, hasNameCtx)
	assetCache := newStaticSiteAssetCachePolicy(construct, nameCtx, hasNameCtx)
	headersPolicy := props.ResponseHeadersPolicy
	if headersPolicy == nil {
		headersPolicy = newStaticSiteResponseHeadersPolicy(construct, props.DomainName)
	}

	defaultBehavior := &awscloudfront.BehaviorOptions{
		Origin:               staticOrigin,
		ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		CachePolicy:          htmlCache,
		ResponseHeadersPolicy: headersPolicy,
		Compress:             jsii.Bool(true),
	}

	errorResponses := []*awscloudfront.ErrorResponse(nil)
	if *props.SinglePageApp {
		errorResponses = []*awscloudfront.ErrorResponse{
			{
				HttpStatus:         jsii.Number(403),
				ResponseHttpStatus: jsii.Number(200),
				ResponsePagePath:   jsii.String("/index.html"),
				Ttl:                awscdk.Duration_Seconds(jsii.Number(0)),
			},
			{
				HttpStatus:         jsii.Number(404),
				ResponseHttpStatus: jsii.Number(200),
				ResponsePagePath:   jsii.String("/index.html"),
				Ttl:                awscdk.Duration_Seconds(jsii.Number(0)),
			},
		}
	}

	frontend.Distribution = awscloudfront.NewDistribution(construct, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		DefaultBehavior:   defaultBehavior,
		DomainNames:       &[]*string{props.DomainName},
		Certificate:       cert,
		DefaultRootObject: jsii.String("index.html"),
		EnableIpv6:        props.EnableIpv6,
		HttpVersion:       props.HttpVersion,
		PriceClass:        props.PriceClass,
		WebAclId:          props.WebAclId,
		ErrorResponses:    &errorResponses,
	})

	if props.Tags != nil {
		for k, v := range *props.Tags {
			awscdk.Tags_Of(frontend.Distribution).Add(jsii.String(k), v, nil)
		}
	}
	if hasNameCtx {
		awscdk.Tags_Of(frontend.Distribution).Add(jsii.String("Application"), jsii.String(nameCtx.AppName), nil)
		awscdk.Tags_Of(frontend.Distribution).Add(jsii.String("Environment"), jsii.String(nameCtx.Stage), nil)
		if nameCtx.Tenant != "" {
			awscdk.Tags_Of(frontend.Distribution).Add(jsii.String("Partner"), jsii.String(nameCtx.Tenant), nil)
		}
	}
	awscdk.Tags_Of(frontend.Distribution).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(frontend.Distribution).Add(jsii.String("Component"), jsii.String("FrontendDistribution"), nil)

	// Long-lived asset behaviors.
	assetPatterns := props.HashedAssetPathPatterns
	if assetPatterns == nil {
		assetPatterns = &[]*string{
			jsii.String("assets/*"),
			jsii.String("static/*"),
			jsii.String("build/*"),
			jsii.String("dist/*"),
			jsii.String("_next/*"),
			jsii.String("_nuxt/*"),
		}
	}
	for _, pattern := range *assetPatterns {
		if pattern == nil || strings.TrimSpace(*pattern) == "" {
			continue
		}
		frontend.Distribution.AddBehavior(pattern, staticOrigin, &awscloudfront.AddBehaviorOptions{
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			CachePolicy:          assetCache,
			ResponseHeadersPolicy: headersPolicy,
			Compress:             jsii.Bool(true),
		})
	}

	// API behaviors (no caching).
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
		apiCachePolicy = defaultAPICachePolicy(construct, nameCtx, hasNameCtx)
	}
	apiOriginRequestPolicy := props.ApiOriginRequestPolicy

	for _, pattern := range *apiPatterns {
		if pattern == nil || strings.TrimSpace(*pattern) == "" {
			continue
		}
		frontend.Distribution.AddBehavior(pattern, apiOrigin, &awscloudfront.AddBehaviorOptions{
			AllowedMethods:      awscloudfront.AllowedMethods_ALLOW_ALL(),
			CachedMethods:       awscloudfront.CachedMethods_CACHE_GET_HEAD_OPTIONS(),
			CachePolicy:         apiCachePolicy,
			OriginRequestPolicy: apiOriginRequestPolicy,
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			Compress:            jsii.Bool(true),
		})
	}

	// DNS records.
	aliasTarget := awsroute53.RecordTarget_FromAlias(awsroute53targets.NewCloudFrontTarget(frontend.Distribution))
	awsroute53.NewARecord(construct, jsii.String("AliasA"), &awsroute53.ARecordProps{
		Zone:       props.HostedZone,
		RecordName: relativeRecordName(props.HostedZone, props.DomainName),
		Target:     aliasTarget,
	})
	if props.EnableIpv6 != nil && *props.EnableIpv6 {
		awsroute53.NewAaaaRecord(construct, jsii.String("AliasAAAA"), &awsroute53.AaaaRecordProps{
			Zone:       props.HostedZone,
			RecordName: relativeRecordName(props.HostedZone, props.DomainName),
			Target:     aliasTarget,
		})
	}

	// Optional www redirect distribution.
	if *props.EnableWWWRedirect {
		www := props.WWWDomainName
		if www == nil || strings.TrimSpace(*www) == "" {
			www = jsii.String("www." + strings.TrimSuffix(strings.TrimSpace(*props.DomainName), "."))
		}
		frontend.WWWRedirect = NewHostRedirect(construct, jsii.String("WWWRedirect"), &HostRedirectProps{
			FromDomainName: www,
			ToDomainName:   props.DomainName,
			HostedZone:     props.HostedZone,
			Certificate:    cert,
			WebAclId:       props.WebAclId,
			EnableIpv6:     props.EnableIpv6,
			Tags:           props.Tags,
		})
	}

	return frontend
}
