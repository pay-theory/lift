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

	"github.com/pay-theory/lift/pkg/naming"
)

// MediaCDNProps defines properties for a media-focused CloudFront distribution.
type MediaCDNProps struct {
	// Required: domain name (e.g., "media.example.com").
	DomainName *string

	// Required: hosted zone authoritative for DomainName.
	HostedZone awsroute53.IHostedZone

	// Optional: stable naming inputs for deterministic bucket names.
	AppName   *string
	Stage     *string
	Partner   *string
	BucketName *string

	// Optional: bucket configuration.
	RemovalPolicy     awscdk.RemovalPolicy
	AutoDeleteObjects *bool
	Versioned         *bool

	// Optional: custom domain certificate. If omitted, Lift creates a DNS-validated certificate in us-east-1.
	Certificate awscertificatemanager.ICertificate

	// Optional: additional SANs to include when Lift creates a certificate.
	SubjectAlternativeNames *[]*string

	// Optional: enable private media via CloudFront Key Groups (signed URLs/cookies).
	EnablePrivateMedia *bool

	// Required when EnablePrivateMedia is true: PEM-encoded public key used by CloudFront for validation.
	PublicKeyEncoded *string

	// Optional: path patterns that require signed URLs/cookies (default: ["private/*"]).
	PrivatePathPatterns *[]*string

	// Optional: WAFv2 Web ACL ARN (global scope) to attach to the distribution.
	WebAclId *string

	// Optional: distribution tuning.
	PriceClass awscloudfront.PriceClass
	HttpVersion awscloudfront.HttpVersion
	EnableIpv6 *bool

	// Optional: override response headers policy.
	ResponseHeadersPolicy awscloudfront.IResponseHeadersPolicy

	// Optional: tags applied to created resources.
	Tags *map[string]*string
}

// MediaCDN creates an S3 + CloudFront distribution tuned for media delivery.
type MediaCDN struct {
	constructs.Construct

	Bucket       awss3.Bucket
	Distribution awscloudfront.Distribution
	Certificate  awscertificatemanager.ICertificate

	// Only set when EnablePrivateMedia is true.
	PublicKey awscloudfront.PublicKey
	KeyGroup  awscloudfront.KeyGroup
}

// NewMediaCDN creates a media distribution backed by a private S3 bucket with OAC.
func NewMediaCDN(scope constructs.Construct, id *string, props *MediaCDNProps) *MediaCDN {
	construct := constructs.NewConstruct(scope, id)
	if props == nil {
		props = &MediaCDNProps{}
	}
	if props.DomainName == nil || strings.TrimSpace(*props.DomainName) == "" {
		panic("MediaCDN requires DomainName")
	}
	if props.HostedZone == nil {
		panic("MediaCDN requires HostedZone")
	}
	if props.EnablePrivateMedia == nil {
		props.EnablePrivateMedia = jsii.Bool(false)
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

	cdn := &MediaCDN{Construct: construct}

	bucketName := resolveS3BucketName(construct, id, props.BucketName, nameCtx, hasNameCtx, "media")
	cdn.Bucket = awss3.NewBucket(construct, jsii.String("Bucket"), &awss3.BucketProps{
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
			awscdk.Tags_Of(cdn.Bucket).Add(jsii.String(k), v, nil)
		}
	}
	if hasNameCtx {
		awscdk.Tags_Of(cdn.Bucket).Add(jsii.String("Application"), jsii.String(nameCtx.AppName), nil)
		awscdk.Tags_Of(cdn.Bucket).Add(jsii.String("Environment"), jsii.String(nameCtx.Stage), nil)
		if nameCtx.Tenant != "" {
			awscdk.Tags_Of(cdn.Bucket).Add(jsii.String("Partner"), jsii.String(nameCtx.Tenant), nil)
		}
	}
	awscdk.Tags_Of(cdn.Bucket).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(cdn.Bucket).Add(jsii.String("Component"), jsii.String("MediaCDN"), nil)

	cert := props.Certificate
	if cert == nil {
		sans := []*string{}
		if props.SubjectAlternativeNames != nil {
			sans = append(sans, (*props.SubjectAlternativeNames)...)
		}
		cert = awscertificatemanager.NewDnsValidatedCertificate(construct, jsii.String("Certificate"), &awscertificatemanager.DnsValidatedCertificateProps{
			DomainName:              props.DomainName,
			HostedZone:              props.HostedZone,
			Region:                  jsii.String("us-east-1"),
			SubjectAlternativeNames: &sans,
		})
	}
	cdn.Certificate = cert

	origin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(cdn.Bucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{})

	headersPolicy := props.ResponseHeadersPolicy
	if headersPolicy == nil {
		headersPolicy = newMediaCDNResponseHeadersPolicy(construct)
	}

	mediaCache := newMediaCDNCachePolicy(construct, nameCtx, hasNameCtx)

	defaultBehavior := &awscloudfront.BehaviorOptions{
		Origin:               origin,
		ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		CachePolicy:          mediaCache,
		ResponseHeadersPolicy: headersPolicy,
		Compress:             jsii.Bool(true),
	}

	cdn.Distribution = awscloudfront.NewDistribution(construct, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		DefaultBehavior: defaultBehavior,
		DomainNames:     &[]*string{props.DomainName},
		Certificate:     cert,
		EnableIpv6:      props.EnableIpv6,
		HttpVersion:     props.HttpVersion,
		PriceClass:      props.PriceClass,
		WebAclId:        props.WebAclId,
	})

	if props.Tags != nil {
		for k, v := range *props.Tags {
			awscdk.Tags_Of(cdn.Distribution).Add(jsii.String(k), v, nil)
		}
	}
	if hasNameCtx {
		awscdk.Tags_Of(cdn.Distribution).Add(jsii.String("Application"), jsii.String(nameCtx.AppName), nil)
		awscdk.Tags_Of(cdn.Distribution).Add(jsii.String("Environment"), jsii.String(nameCtx.Stage), nil)
		if nameCtx.Tenant != "" {
			awscdk.Tags_Of(cdn.Distribution).Add(jsii.String("Partner"), jsii.String(nameCtx.Tenant), nil)
		}
	}
	awscdk.Tags_Of(cdn.Distribution).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(cdn.Distribution).Add(jsii.String("Component"), jsii.String("MediaCDN"), nil)

	// Optional private behavior(s).
	if *props.EnablePrivateMedia {
		if props.PublicKeyEncoded == nil || strings.TrimSpace(*props.PublicKeyEncoded) == "" {
			panic("MediaCDN EnablePrivateMedia requires PublicKeyEncoded (PEM)")
		}

		keyGroupName := (*string)(nil)
		if hasNameCtx {
			keyGroupName = jsii.String(nameCtx.ResourceName("media-keygroup"))
		}

		cdn.PublicKey = awscloudfront.NewPublicKey(construct, jsii.String("PublicKey"), &awscloudfront.PublicKeyProps{
			EncodedKey: props.PublicKeyEncoded,
		})
		cdn.KeyGroup = awscloudfront.NewKeyGroup(construct, jsii.String("KeyGroup"), &awscloudfront.KeyGroupProps{
			KeyGroupName: keyGroupName,
			Items:        &[]awscloudfront.IPublicKeyRef{cdn.PublicKey},
		})

		privatePatterns := props.PrivatePathPatterns
		if privatePatterns == nil {
			privatePatterns = &[]*string{jsii.String("private/*")}
		}
		for _, pattern := range *privatePatterns {
			if pattern == nil || strings.TrimSpace(*pattern) == "" {
				continue
			}
			cdn.Distribution.AddBehavior(pattern, origin, &awscloudfront.AddBehaviorOptions{
				ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
				CachePolicy:          mediaCache,
				ResponseHeadersPolicy: headersPolicy,
				TrustedKeyGroups:     &[]awscloudfront.IKeyGroupRef{cdn.KeyGroup},
				Compress:             jsii.Bool(true),
			})
		}
	}

	// DNS records.
	aliasTarget := awsroute53.RecordTarget_FromAlias(awsroute53targets.NewCloudFrontTarget(cdn.Distribution))
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

	return cdn
}

func newMediaCDNCachePolicy(scope constructs.Construct, nameCtx naming.Context, hasNameCtx bool) awscloudfront.CachePolicy {
	name := (*string)(nil)
	if hasNameCtx {
		name = jsii.String(nameCtx.ResourceName("media-cache"))
	}

	return awscloudfront.NewCachePolicy(scope, jsii.String("MediaCachePolicy"), &awscloudfront.CachePolicyProps{
		CachePolicyName: name,
		Comment:         jsii.String("Lift media cache policy"),
		CookieBehavior:  awscloudfront.CacheCookieBehavior_None(),
		HeaderBehavior:  awscloudfront.CacheHeaderBehavior_None(),
		QueryStringBehavior: awscloudfront.CacheQueryStringBehavior_None(),
		EnableAcceptEncodingBrotli: jsii.Bool(true),
		EnableAcceptEncodingGzip:   jsii.Bool(true),
		MinTtl:     awscdk.Duration_Seconds(jsii.Number(0)),
		DefaultTtl: awscdk.Duration_Days(jsii.Number(7)),
		MaxTtl:     awscdk.Duration_Days(jsii.Number(30)),
	})
}

func newMediaCDNResponseHeadersPolicy(scope constructs.Construct) awscloudfront.ResponseHeadersPolicy {
	return awscloudfront.NewResponseHeadersPolicy(scope, jsii.String("MediaResponseHeadersPolicy"), &awscloudfront.ResponseHeadersPolicyProps{
		Comment: jsii.String("Lift media CDN security headers"),
		SecurityHeadersBehavior: &awscloudfront.ResponseSecurityHeadersBehavior{
			ContentTypeOptions: &awscloudfront.ResponseHeadersContentTypeOptions{Override: jsii.Bool(true)},
			StrictTransportSecurity: &awscloudfront.ResponseHeadersStrictTransportSecurity{
				AccessControlMaxAge: awscdk.Duration_Days(jsii.Number(365)),
				IncludeSubdomains:   jsii.Bool(true),
				Override:            jsii.Bool(true),
			},
		},
		RemoveHeaders: &[]*string{jsii.String("Server")},
	})
}
