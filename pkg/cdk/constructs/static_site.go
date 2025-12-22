package constructs

import (
	"fmt"
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

	"github.com/pay-theory/lift/pkg/naming"
)

// StaticSiteProps defines properties for a CloudFront-backed static site.
type StaticSiteProps struct {
	// Required: apex/canonical domain (e.g., "example.com").
	DomainName *string

	// Required: hosted zone authoritative for DomainName.
	HostedZone awsroute53.IHostedZone

	// Optional: stable naming inputs for deterministic bucket names.
	AppName  *string
	Stage    *string
	Partner  *string
	BucketName *string

	// Optional: bucket configuration.
	RemovalPolicy     awscdk.RemovalPolicy
	AutoDeleteObjects *bool
	Versioned         *bool

	// Optional: enable access logs (log bucket auto-created unless provided).
	EnableAccessLogs *bool
	AccessLogsBucket awss3.IBucket
	AccessLogsPrefix *string

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

	// Optional: override response headers policy.
	ResponseHeadersPolicy awscloudfront.IResponseHeadersPolicy

	// Optional: tags applied to created resources.
	Tags *map[string]*string
}

// StaticSite creates an S3 + CloudFront + Route53 static site with OAC and safe defaults.
type StaticSite struct {
	constructs.Construct

	Bucket       awss3.Bucket
	Distribution awscloudfront.Distribution
	Certificate  awscertificatemanager.ICertificate

	// WWWRedirect is set when EnableWWWRedirect is true.
	WWWRedirect *HostRedirect
}

// NewStaticSite creates a CloudFront distribution backed by a private S3 bucket.
func NewStaticSite(scope constructs.Construct, id *string, props *StaticSiteProps) *StaticSite {
	construct := constructs.NewConstruct(scope, id)
	if props == nil {
		props = &StaticSiteProps{}
	}
	if props.DomainName == nil || strings.TrimSpace(*props.DomainName) == "" {
		panic("StaticSite requires DomainName")
	}
	if props.HostedZone == nil {
		panic("StaticSite requires HostedZone")
	}

	if props.EnableWWWRedirect == nil {
		props.EnableWWWRedirect = jsii.Bool(true)
	}
	if props.SinglePageApp == nil {
		props.SinglePageApp = jsii.Bool(false)
	}
	if props.EnableAccessLogs == nil {
		props.EnableAccessLogs = jsii.Bool(false)
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

	site := &StaticSite{Construct: construct}

	bucketName := resolveS3BucketName(construct, id, props.BucketName, nameCtx, hasNameCtx, "static-site")
	site.Bucket = awss3.NewBucket(construct, jsii.String("Bucket"), &awss3.BucketProps{
		BucketName:       bucketName,
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EnforceSSL:       jsii.Bool(true),
		ObjectOwnership:  awss3.ObjectOwnership_BUCKET_OWNER_ENFORCED,
		RemovalPolicy:    props.RemovalPolicy,
		AutoDeleteObjects: props.AutoDeleteObjects,
		Versioned:        props.Versioned,
		Encryption:       awss3.BucketEncryption_S3_MANAGED,
	})

	if props.Tags != nil {
		for k, v := range *props.Tags {
			awscdk.Tags_Of(site.Bucket).Add(jsii.String(k), v, nil)
		}
	}
	if hasNameCtx {
		awscdk.Tags_Of(site.Bucket).Add(jsii.String("Application"), jsii.String(nameCtx.AppName), nil)
		awscdk.Tags_Of(site.Bucket).Add(jsii.String("Environment"), jsii.String(nameCtx.Stage), nil)
		if nameCtx.Tenant != "" {
			awscdk.Tags_Of(site.Bucket).Add(jsii.String("Partner"), jsii.String(nameCtx.Tenant), nil)
		}
	}
	awscdk.Tags_Of(site.Bucket).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(site.Bucket).Add(jsii.String("Component"), jsii.String("StaticSite"), nil)

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
	site.Certificate = cert

	origin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(site.Bucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{})

	htmlCache := newStaticSiteHTMLCachePolicy(construct, nameCtx, hasNameCtx)
	assetCache := newStaticSiteAssetCachePolicy(construct, nameCtx, hasNameCtx)
	headersPolicy := props.ResponseHeadersPolicy
	if headersPolicy == nil {
		headersPolicy = newStaticSiteResponseHeadersPolicy(construct, props.DomainName)
	}

	defaultBehavior := &awscloudfront.BehaviorOptions{
		Origin:               origin,
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

	site.Distribution = awscloudfront.NewDistribution(construct, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		DefaultBehavior: defaultBehavior,
		DomainNames:     &[]*string{props.DomainName},
		Certificate:     cert,
		DefaultRootObject: jsii.String("index.html"),
		EnableIpv6:      props.EnableIpv6,
		HttpVersion:     props.HttpVersion,
		PriceClass:      props.PriceClass,
		WebAclId:        props.WebAclId,
		ErrorResponses:  &errorResponses,
		EnableLogging:   props.EnableAccessLogs,
		LogBucket:       resolveStaticSiteLogBucket(construct, id, props, nameCtx, hasNameCtx),
		LogFilePrefix:   props.AccessLogsPrefix,
	})

	if props.Tags != nil {
		for k, v := range *props.Tags {
			awscdk.Tags_Of(site.Distribution).Add(jsii.String(k), v, nil)
		}
	}
	if hasNameCtx {
		awscdk.Tags_Of(site.Distribution).Add(jsii.String("Application"), jsii.String(nameCtx.AppName), nil)
		awscdk.Tags_Of(site.Distribution).Add(jsii.String("Environment"), jsii.String(nameCtx.Stage), nil)
		if nameCtx.Tenant != "" {
			awscdk.Tags_Of(site.Distribution).Add(jsii.String("Partner"), jsii.String(nameCtx.Tenant), nil)
		}
	}
	awscdk.Tags_Of(site.Distribution).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(site.Distribution).Add(jsii.String("Component"), jsii.String("StaticSite"), nil)

	// DNS records.
	aliasTarget := awsroute53.RecordTarget_FromAlias(awsroute53targets.NewCloudFrontTarget(site.Distribution))
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
		site.Distribution.AddBehavior(pattern, origin, &awscloudfront.AddBehaviorOptions{
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			CachePolicy:          assetCache,
			ResponseHeadersPolicy: headersPolicy,
			Compress:             jsii.Bool(true),
		})
	}

	// Optional www redirect distribution.
	if *props.EnableWWWRedirect {
		www := props.WWWDomainName
		if www == nil || strings.TrimSpace(*www) == "" {
			www = jsii.String("www." + strings.TrimSuffix(strings.TrimSpace(*props.DomainName), "."))
		}
		site.WWWRedirect = NewHostRedirect(construct, jsii.String("WWWRedirect"), &HostRedirectProps{
			FromDomainName: www,
			ToDomainName:   props.DomainName,
			HostedZone:     props.HostedZone,
			Certificate:    cert,
			WebAclId:       props.WebAclId,
			EnableIpv6:     props.EnableIpv6,
			Tags:           props.Tags,
		})
	}

	return site
}

func resolveStaticSiteLogBucket(scope constructs.Construct, id *string, props *StaticSiteProps, nameCtx naming.Context, hasNameCtx bool) awss3.IBucket {
	if props == nil {
		return nil
	}
	if props.EnableAccessLogs == nil || !*props.EnableAccessLogs {
		return nil
	}
	if props.AccessLogsBucket != nil {
		return props.AccessLogsBucket
	}

	bucketName := resolveS3BucketName(scope, jsii.String(*id+"Logs"), nil, nameCtx, hasNameCtx, "static-site-logs")
	return awss3.NewBucket(scope, jsii.String("AccessLogsBucket"), &awss3.BucketProps{
		BucketName:        bucketName,
		BlockPublicAccess: awss3.BlockPublicAccess_BLOCK_ALL(),
		EnforceSSL:        jsii.Bool(true),
		ObjectOwnership:   awss3.ObjectOwnership_OBJECT_WRITER,
		RemovalPolicy:     awscdk.RemovalPolicy_RETAIN,
		Encryption:        awss3.BucketEncryption_S3_MANAGED,
	})
}

func newStaticSiteHTMLCachePolicy(scope constructs.Construct, nameCtx naming.Context, hasNameCtx bool) awscloudfront.CachePolicy {
	name := (*string)(nil)
	if hasNameCtx {
		name = jsii.String(fmt.Sprintf("%s-html", nameCtx.ResourceName("static-site-cache")))
	}

	return awscloudfront.NewCachePolicy(scope, jsii.String("HTMLCachePolicy"), &awscloudfront.CachePolicyProps{
		CachePolicyName: name,
		Comment:         jsii.String("Lift static site HTML cache policy"),
		CookieBehavior:  awscloudfront.CacheCookieBehavior_None(),
		HeaderBehavior:  awscloudfront.CacheHeaderBehavior_None(),
		QueryStringBehavior: awscloudfront.CacheQueryStringBehavior_None(),
		EnableAcceptEncodingBrotli: jsii.Bool(true),
		EnableAcceptEncodingGzip:   jsii.Bool(true),
		MinTtl:     awscdk.Duration_Seconds(jsii.Number(0)),
		DefaultTtl: awscdk.Duration_Minutes(jsii.Number(1)),
		MaxTtl:     awscdk.Duration_Minutes(jsii.Number(5)),
	})
}

func newStaticSiteAssetCachePolicy(scope constructs.Construct, nameCtx naming.Context, hasNameCtx bool) awscloudfront.CachePolicy {
	name := (*string)(nil)
	if hasNameCtx {
		name = jsii.String(fmt.Sprintf("%s-assets", nameCtx.ResourceName("static-site-cache")))
	}

	return awscloudfront.NewCachePolicy(scope, jsii.String("AssetCachePolicy"), &awscloudfront.CachePolicyProps{
		CachePolicyName: name,
		Comment:         jsii.String("Lift static site hashed asset cache policy"),
		CookieBehavior:  awscloudfront.CacheCookieBehavior_None(),
		HeaderBehavior:  awscloudfront.CacheHeaderBehavior_None(),
		QueryStringBehavior: awscloudfront.CacheQueryStringBehavior_None(),
		EnableAcceptEncodingBrotli: jsii.Bool(true),
		EnableAcceptEncodingGzip:   jsii.Bool(true),
		MinTtl:     awscdk.Duration_Days(jsii.Number(1)),
		DefaultTtl: awscdk.Duration_Days(jsii.Number(365)),
		MaxTtl:     awscdk.Duration_Days(jsii.Number(365)),
	})
}

func newStaticSiteResponseHeadersPolicy(scope constructs.Construct, domainName *string) awscloudfront.ResponseHeadersPolicy {
	domain := ""
	if domainName != nil {
		domain = strings.TrimSuffix(strings.TrimSpace(*domainName), ".")
	}
	connect := []string{"'self'"}
	if domain != "" {
		connect = append(connect, "https://api."+domain, "wss://ws."+domain)
	}

	csp := strings.Join([]string{
		"default-src 'self'",
		"base-uri 'self'",
		"object-src 'none'",
		"frame-ancestors 'none'",
		"img-src 'self' data: https:",
		"font-src 'self' data: https:",
		"style-src 'self' 'unsafe-inline'",
		"script-src 'self' 'unsafe-inline'",
		"connect-src " + strings.Join(connect, " "),
	}, "; ") + ";"

	return awscloudfront.NewResponseHeadersPolicy(scope, jsii.String("ResponseHeadersPolicy"), &awscloudfront.ResponseHeadersPolicyProps{
		Comment: jsii.String("Lift static site security headers"),
		SecurityHeadersBehavior: &awscloudfront.ResponseSecurityHeadersBehavior{
			ContentSecurityPolicy: &awscloudfront.ResponseHeadersContentSecurityPolicy{
				ContentSecurityPolicy: jsii.String(csp),
				Override:              jsii.Bool(true),
			},
			ContentTypeOptions: &awscloudfront.ResponseHeadersContentTypeOptions{Override: jsii.Bool(true)},
			FrameOptions: &awscloudfront.ResponseHeadersFrameOptions{
				FrameOption: awscloudfront.HeadersFrameOption_DENY,
				Override:    jsii.Bool(true),
			},
			ReferrerPolicy: &awscloudfront.ResponseHeadersReferrerPolicy{
				ReferrerPolicy: awscloudfront.HeadersReferrerPolicy_STRICT_ORIGIN_WHEN_CROSS_ORIGIN,
				Override:       jsii.Bool(true),
			},
			StrictTransportSecurity: &awscloudfront.ResponseHeadersStrictTransportSecurity{
				AccessControlMaxAge: awscdk.Duration_Days(jsii.Number(365)),
				IncludeSubdomains:   jsii.Bool(true),
				Override:            jsii.Bool(true),
			},
			XssProtection: &awscloudfront.ResponseHeadersXSSProtection{
				Protection: jsii.Bool(true),
				ModeBlock:  jsii.Bool(true),
				Override:   jsii.Bool(true),
			},
		},
		CustomHeadersBehavior: &awscloudfront.ResponseCustomHeadersBehavior{
			CustomHeaders: &[]*awscloudfront.ResponseCustomHeader{
				{
					Header:   jsii.String("Permissions-Policy"),
					Value:    jsii.String("camera=(), microphone=(), geolocation=(), payment=()"),
					Override: jsii.Bool(true),
				},
			},
		},
		RemoveHeaders: &[]*string{
			jsii.String("Server"),
		},
	})
}
