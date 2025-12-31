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

// MediaCDNProps defines properties for a media-focused CloudFront distribution.
type MediaCDNProps struct {
	// Required: hosted zone authoritative for DomainName.
	HostedZone awsroute53.IHostedZone
	// Optional: custom domain certificate. If omitted, Lift creates a DNS-validated certificate in us-east-1.
	Certificate awscertificatemanager.ICertificate
	// Optional: override response headers policy.
	ResponseHeadersPolicy awscloudfront.IResponseHeadersPolicy

	// Required: domain name (e.g., "media.example.com").
	DomainName *string

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

	// Optional: enable private media via CloudFront Key Groups (signed URLs/cookies).
	EnablePrivateMedia *bool

	// Required when EnablePrivateMedia is true: PEM-encoded public key used by CloudFront for validation.
	PublicKeyEncoded *string

	// Optional: path patterns that require signed URLs/cookies (default: ["private/*"]).
	PrivatePathPatterns *[]*string

	// Optional: WAFv2 Web ACL ARN (global scope) to attach to the distribution.
	WebAclId *string

	// Optional: distribution tuning.
	EnableIpv6 *bool

	// Optional: tags applied to created resources.
	Tags *map[string]*string

	// Optional: bucket configuration.
	RemovalPolicy awscdk.RemovalPolicy

	// Optional: distribution tuning.
	PriceClass  awscloudfront.PriceClass
	HttpVersion awscloudfront.HttpVersion
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
	props = normalizeMediaCDNProps(props)

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

	applyStandardConstructTags(cdn.Bucket, props.Tags, nameCtx, hasNameCtx, "MediaCDN")

	cdn.Certificate = resolveMediaCDNCertificate(construct, props)

	origin := awscloudfrontorigins.S3BucketOrigin_WithOriginAccessControl(cdn.Bucket, &awscloudfrontorigins.S3BucketOriginWithOACProps{})

	headersPolicy := resolveMediaCDNHeadersPolicy(construct, props)

	mediaCache := newMediaCDNCachePolicy(construct, nameCtx, hasNameCtx)

	defaultBehavior := &awscloudfront.BehaviorOptions{
		Origin:                origin,
		ViewerProtocolPolicy:  awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
		CachePolicy:           mediaCache,
		ResponseHeadersPolicy: headersPolicy,
		Compress:              jsii.Bool(true),
	}

	cdn.Distribution = awscloudfront.NewDistribution(construct, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		DefaultBehavior: defaultBehavior,
		DomainNames:     &[]*string{props.DomainName},
		Certificate:     cdn.Certificate,
		EnableIpv6:      props.EnableIpv6,
		HttpVersion:     props.HttpVersion,
		PriceClass:      props.PriceClass,
		WebAclId:        props.WebAclId,
	})

	applyStandardConstructTags(cdn.Distribution, props.Tags, nameCtx, hasNameCtx, "MediaCDN")
	configureMediaCDNPrivateBehaviors(construct, cdn, props, origin, mediaCache, headersPolicy, nameCtx, hasNameCtx)
	addCloudFrontAliasRecords(construct, props.HostedZone, props.DomainName, cdn.Distribution, props.EnableIpv6)

	return cdn
}

func normalizeMediaCDNProps(props *MediaCDNProps) *MediaCDNProps {
	if props == nil {
		props = &MediaCDNProps{}
	}
	if props.DomainName == nil || strings.TrimSpace(*props.DomainName) == "" {
		panic("MediaCDN requires DomainName")
	}
	if props.HostedZone == nil {
		panic("MediaCDN requires HostedZone")
	}

	ensureBool(&props.EnablePrivateMedia, false)
	ensureBool(&props.EnableIpv6, true)
	ensureHttpVersion(&props.HttpVersion, awscloudfront.HttpVersion_HTTP2)
	ensurePriceClass(&props.PriceClass, awscloudfront.PriceClass_PRICE_CLASS_ALL)
	ensureRemovalPolicy(&props.RemovalPolicy, awscdk.RemovalPolicy_RETAIN)
	ensureBool(&props.Versioned, false)
	defaultAutoDeleteObjectsIfDestroy(props.RemovalPolicy, &props.AutoDeleteObjects)

	return props
}

func resolveMediaCDNCertificate(scope constructs.Construct, props *MediaCDNProps) awscertificatemanager.ICertificate {
	if props.Certificate != nil {
		return props.Certificate
	}

	sans := []*string{}
	if props.SubjectAlternativeNames != nil {
		sans = append(sans, (*props.SubjectAlternativeNames)...)
	}
	return ensureCloudFrontCertificate(scope, nil, props.DomainName, props.HostedZone, sans)
}

func resolveMediaCDNHeadersPolicy(scope constructs.Construct, props *MediaCDNProps) awscloudfront.IResponseHeadersPolicy {
	if props.ResponseHeadersPolicy != nil {
		return props.ResponseHeadersPolicy
	}
	return newMediaCDNResponseHeadersPolicy(scope)
}

func configureMediaCDNPrivateBehaviors(scope constructs.Construct, cdn *MediaCDN, props *MediaCDNProps, origin awscloudfront.IOrigin, cache awscloudfront.ICachePolicy, headers awscloudfront.IResponseHeadersPolicy, nameCtx naming.Context, hasNameCtx bool) {
	if props.EnablePrivateMedia == nil || !*props.EnablePrivateMedia {
		return
	}
	if props.PublicKeyEncoded == nil || strings.TrimSpace(*props.PublicKeyEncoded) == "" {
		panic("MediaCDN EnablePrivateMedia requires PublicKeyEncoded (PEM)")
	}

	keyGroupName := (*string)(nil)
	if hasNameCtx {
		keyGroupName = jsii.String(nameCtx.ResourceName("media-keygroup"))
	}

	cdn.PublicKey = awscloudfront.NewPublicKey(scope, jsii.String("PublicKey"), &awscloudfront.PublicKeyProps{
		EncodedKey: props.PublicKeyEncoded,
	})
	cdn.KeyGroup = awscloudfront.NewKeyGroup(scope, jsii.String("KeyGroup"), &awscloudfront.KeyGroupProps{
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
			ViewerProtocolPolicy:  awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			CachePolicy:           cache,
			ResponseHeadersPolicy: headers,
			TrustedKeyGroups:      &[]awscloudfront.IKeyGroupRef{cdn.KeyGroup},
			Compress:              jsii.Bool(true),
		})
	}
}

func newMediaCDNCachePolicy(scope constructs.Construct, nameCtx naming.Context, hasNameCtx bool) awscloudfront.CachePolicy {
	name := (*string)(nil)
	if hasNameCtx {
		name = jsii.String(nameCtx.ResourceName("media-cache"))
	}

	return awscloudfront.NewCachePolicy(scope, jsii.String("MediaCachePolicy"), &awscloudfront.CachePolicyProps{
		CachePolicyName:            name,
		Comment:                    jsii.String("Lift media cache policy"),
		CookieBehavior:             awscloudfront.CacheCookieBehavior_None(),
		HeaderBehavior:             awscloudfront.CacheHeaderBehavior_None(),
		QueryStringBehavior:        awscloudfront.CacheQueryStringBehavior_None(),
		EnableAcceptEncodingBrotli: jsii.Bool(true),
		EnableAcceptEncodingGzip:   jsii.Bool(true),
		MinTtl:                     awscdk.Duration_Seconds(jsii.Number(0)),
		DefaultTtl:                 awscdk.Duration_Days(jsii.Number(7)),
		MaxTtl:                     awscdk.Duration_Days(jsii.Number(30)),
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
