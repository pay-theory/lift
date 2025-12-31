package constructs

import (
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscertificatemanager"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53targets"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"github.com/pay-theory/lift/pkg/naming"
)

func ensureBool(v **bool, def bool) {
	if *v == nil {
		*v = jsii.Bool(def)
	}
}

func ensureRemovalPolicy(v *awscdk.RemovalPolicy, def awscdk.RemovalPolicy) {
	if *v == "" {
		*v = def
	}
}

func ensureHttpVersion(v *awscloudfront.HttpVersion, def awscloudfront.HttpVersion) {
	if *v == "" {
		*v = def
	}
}

func ensurePriceClass(v *awscloudfront.PriceClass, def awscloudfront.PriceClass) {
	if *v == "" {
		*v = def
	}
}

func defaultAutoDeleteObjectsIfDestroy(removalPolicy awscdk.RemovalPolicy, autoDelete **bool) {
	if removalPolicy != awscdk.RemovalPolicy_DESTROY {
		return
	}
	if *autoDelete == nil {
		*autoDelete = jsii.Bool(true)
	}
}

func applyStandardConstructTags(target constructs.IConstruct, tags *map[string]*string, nameCtx naming.Context, hasNameCtx bool, component string) {
	if tags != nil {
		for k, v := range *tags {
			awscdk.Tags_Of(target).Add(jsii.String(k), v, nil)
		}
	}
	if hasNameCtx {
		awscdk.Tags_Of(target).Add(jsii.String("Application"), jsii.String(nameCtx.AppName), nil)
		awscdk.Tags_Of(target).Add(jsii.String("Environment"), jsii.String(nameCtx.Stage), nil)
		if nameCtx.Tenant != "" {
			awscdk.Tags_Of(target).Add(jsii.String("Partner"), jsii.String(nameCtx.Tenant), nil)
		}
	}
	awscdk.Tags_Of(target).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(target).Add(jsii.String("Component"), jsii.String(component), nil)
}

func ensureCloudFrontCertificate(scope constructs.Construct, existing awscertificatemanager.ICertificate, domainName *string, hostedZone awsroute53.IHostedZone, sans []*string) awscertificatemanager.ICertificate {
	if existing != nil {
		return existing
	}

	return awscertificatemanager.NewDnsValidatedCertificate(scope, jsii.String("Certificate"), &awscertificatemanager.DnsValidatedCertificateProps{ //nolint:staticcheck // Required for CloudFront cross-region support until CDK supports it natively in Certificate
		DomainName:              domainName,
		HostedZone:              hostedZone,
		Region:                  jsii.String("us-east-1"),
		SubjectAlternativeNames: &sans,
	})
}

func resolveWWWDomainName(domainName *string, wwwDomainName *string) *string {
	if wwwDomainName != nil && strings.TrimSpace(*wwwDomainName) != "" {
		return wwwDomainName
	}
	return jsii.String("www." + strings.TrimSuffix(strings.TrimSpace(*domainName), "."))
}

func errorResponsesForSinglePageApp(enabled *bool) []*awscloudfront.ErrorResponse {
	if enabled == nil || !*enabled {
		return nil
	}

	return []*awscloudfront.ErrorResponse{
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

func defaultHashedAssetPatterns(patterns *[]*string) *[]*string {
	if patterns != nil {
		return patterns
	}
	return &[]*string{
		jsii.String("assets/*"),
		jsii.String("static/*"),
		jsii.String("build/*"),
		jsii.String("dist/*"),
		jsii.String("_next/*"),
		jsii.String("_nuxt/*"),
	}
}

func addHashedAssetBehaviors(distribution awscloudfront.Distribution, origin awscloudfront.IOrigin, patterns *[]*string, cache awscloudfront.ICachePolicy, headers awscloudfront.IResponseHeadersPolicy) {
	patterns = defaultHashedAssetPatterns(patterns)
	for _, pattern := range *patterns {
		if pattern == nil || strings.TrimSpace(*pattern) == "" {
			continue
		}
		distribution.AddBehavior(pattern, origin, &awscloudfront.AddBehaviorOptions{
			ViewerProtocolPolicy:  awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			CachePolicy:           cache,
			ResponseHeadersPolicy: headers,
			Compress:              jsii.Bool(true),
		})
	}
}

func addCloudFrontAliasRecords(scope constructs.Construct, zone awsroute53.IHostedZone, domainName *string, distribution awscloudfront.Distribution, enableIpv6 *bool) {
	aliasTarget := awsroute53.RecordTarget_FromAlias(awsroute53targets.NewCloudFrontTarget(distribution))
	awsroute53.NewARecord(scope, jsii.String("AliasA"), &awsroute53.ARecordProps{
		Zone:       zone,
		RecordName: relativeRecordName(zone, domainName),
		Target:     aliasTarget,
	})

	if enableIpv6 != nil && *enableIpv6 {
		awsroute53.NewAaaaRecord(scope, jsii.String("AliasAAAA"), &awsroute53.AaaaRecordProps{
			Zone:       zone,
			RecordName: relativeRecordName(zone, domainName),
			Target:     aliasTarget,
		})
	}
}
