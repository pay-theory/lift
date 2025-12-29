package constructs

import (
	"strings"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awscloudfront"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsroute53"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"

	"github.com/pay-theory/lift/pkg/naming"
)

func cloudfrontNamingContext(appName, stage, partner *string) (naming.Context, bool) {
	ctx := naming.Context{}
	if appName != nil {
		ctx.AppName = strings.TrimSpace(*appName)
	}
	if stage != nil {
		ctx.Stage = strings.TrimSpace(*stage)
	}
	if partner != nil {
		ctx.Tenant = strings.TrimSpace(*partner)
	}
	if !ctx.IsComplete() {
		return naming.Context{}, false
	}
	return ctx.Normalize(), true
}

func resolveS3BucketName(scope constructs.IConstruct, id *string, explicitName *string, nameCtx naming.Context, hasNameCtx bool, resource string) *string {
	if explicitName != nil && *explicitName != "" {
		safe := naming.SanitizeS3BucketName(*explicitName)
		if safe != *explicitName {
			panic("invalid S3 bucket name; provide a lowercase, DNS-safe name")
		}
		return explicitName
	}

	if hasNameCtx {
		return jsii.String(nameCtx.S3BucketName(resource))
	}

	stack := awscdk.Stack_Of(scope)
	fallback := naming.SanitizeS3BucketName(*stack.StackName() + "-" + *id + "-" + resource)
	return jsii.String(fallback)
}

func relativeRecordName(zone awsroute53.IHostedZone, fqdn *string) *string {
	if fqdn == nil || *fqdn == "" {
		return nil
	}

	name := strings.TrimSuffix(strings.TrimSpace(*fqdn), ".")
	zoneName := strings.TrimSuffix(strings.TrimSpace(*zone.ZoneName()), ".")

	if strings.EqualFold(name, zoneName) {
		return nil
	}

	suffix := "." + zoneName
	if strings.HasSuffix(strings.ToLower(name), strings.ToLower(suffix)) {
		rel := strings.TrimSuffix(name, suffix)
		rel = strings.TrimSuffix(rel, ".")
		if rel == "" {
			return nil
		}
		return jsii.String(rel)
	}

	return jsii.String(name)
}

func defaultAPIOriginRequestPolicy(scope constructs.Construct, nameCtx naming.Context, hasNameCtx bool) awscloudfront.OriginRequestPolicy {
	name := (*string)(nil)
	if hasNameCtx {
		name = jsii.String(nameCtx.ResourceName("api-origin-request"))
	}

	return awscloudfront.NewOriginRequestPolicy(scope, jsii.String("ApiOriginRequestPolicy"), &awscloudfront.OriginRequestPolicyProps{
		OriginRequestPolicyName: name,
		Comment:                 jsii.String("Lift API origin request policy (query strings + allowlisted headers)"),
		CookieBehavior:          awscloudfront.OriginRequestCookieBehavior_None(),
		QueryStringBehavior:     awscloudfront.OriginRequestQueryStringBehavior_All(),
		HeaderBehavior: awscloudfront.OriginRequestHeaderBehavior_AllowList(
			jsii.String("Content-Type"),
			jsii.String("Accept"),
			jsii.String("Accept-Language"),
			jsii.String("Origin"),
			jsii.String("Access-Control-Request-Method"),
			jsii.String("Access-Control-Request-Headers"),
		),
	})
}

func defaultAPICachePolicy(scope constructs.Construct, nameCtx naming.Context, hasNameCtx bool) awscloudfront.CachePolicy {
	name := (*string)(nil)
	if hasNameCtx {
		name = jsii.String(nameCtx.ResourceName("api-cache"))
	}

	return awscloudfront.NewCachePolicy(scope, jsii.String("ApiCachePolicy"), &awscloudfront.CachePolicyProps{
		CachePolicyName:     name,
		Comment:             jsii.String("Lift API cache policy (default TTL 0; forwards auth + CORS headers)"),
		CookieBehavior:      awscloudfront.CacheCookieBehavior_None(),
		QueryStringBehavior: awscloudfront.CacheQueryStringBehavior_All(),
		HeaderBehavior: awscloudfront.CacheHeaderBehavior_AllowList(
			jsii.String("Authorization"),
			jsii.String("Content-Type"),
			jsii.String("Accept"),
			jsii.String("Accept-Language"),
			jsii.String("Origin"),
			jsii.String("Access-Control-Request-Method"),
			jsii.String("Access-Control-Request-Headers"),
		),
		EnableAcceptEncodingBrotli: jsii.Bool(true),
		EnableAcceptEncodingGzip:   jsii.Bool(true),
		MinTtl:                     awscdk.Duration_Seconds(jsii.Number(0)),
		DefaultTtl:                 awscdk.Duration_Seconds(jsii.Number(0)),
		MaxTtl:                     awscdk.Duration_Seconds(jsii.Number(1)),
	})
}
