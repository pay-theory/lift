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
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

// HostRedirectProps defines properties for a CloudFront Function-based host redirect.
type HostRedirectProps struct {
	// Required: source host (e.g., "www.example.com").
	FromDomainName *string
	// Required: target host (e.g., "example.com").
	ToDomainName *string
	// Required: hosted zone authoritative for FromDomainName.
	HostedZone awsroute53.IHostedZone

	// Optional: certificate for FromDomainName. If omitted, Lift creates a DNS-validated certificate in us-east-1.
	Certificate awscertificatemanager.ICertificate

	// Optional: WAFv2 Web ACL ARN (global scope).
	WebAclId *string

	// Optional: create AAAA alias record (default true).
	EnableIpv6 *bool

	// Optional: tags applied to created resources.
	Tags *map[string]*string
}

// HostRedirect implements a "www -> apex" (or any host -> host) redirect using CloudFront Functions.
type HostRedirect struct {
	constructs.Construct

	Distribution awscloudfront.Distribution
	Function     awscloudfront.Function
	Certificate  awscertificatemanager.ICertificate
}

// NewHostRedirect creates a redirect-only CloudFront distribution returning a 308 and preserving path+query.
func NewHostRedirect(scope constructs.Construct, id *string, props *HostRedirectProps) *HostRedirect {
	construct := constructs.NewConstruct(scope, id)
	if props == nil {
		props = &HostRedirectProps{}
	}
	if props.FromDomainName == nil || strings.TrimSpace(*props.FromDomainName) == "" {
		panic("HostRedirect requires FromDomainName")
	}
	if props.ToDomainName == nil || strings.TrimSpace(*props.ToDomainName) == "" {
		panic("HostRedirect requires ToDomainName")
	}
	if strings.EqualFold(strings.TrimSuffix(strings.TrimSpace(*props.FromDomainName), "."), strings.TrimSuffix(strings.TrimSpace(*props.ToDomainName), ".")) {
		panic("HostRedirect FromDomainName and ToDomainName must differ")
	}
	if props.HostedZone == nil {
		panic("HostRedirect requires HostedZone")
	}
	if props.EnableIpv6 == nil {
		props.EnableIpv6 = jsii.Bool(true)
	}

	redirect := &HostRedirect{Construct: construct}

	cert := props.Certificate
	if cert == nil {
		cert = awscertificatemanager.NewDnsValidatedCertificate(construct, jsii.String("Certificate"), &awscertificatemanager.DnsValidatedCertificateProps{ //nolint:staticcheck // Required for CloudFront cross-region support
			DomainName: props.FromDomainName,
			HostedZone: props.HostedZone,
			Region:     jsii.String("us-east-1"),
		})
	}
	redirect.Certificate = cert

	code := fmt.Sprintf(`function handler(event) {
  var request = event.request;
  var uri = request.uri || "/";
  var qs = request.querystring;
  var query = "";
  if (qs && Object.keys(qs).length > 0) {
    var parts = [];
    for (var key in qs) {
      if (!Object.prototype.hasOwnProperty.call(qs, key)) continue;
      var v = qs[key];
      if (v && v.multiValue) {
        for (var i = 0; i < v.multiValue.length; i++) {
          parts.push(encodeURIComponent(key) + "=" + encodeURIComponent(v.multiValue[i].value));
        }
      } else if (v && v.value !== undefined) {
        parts.push(encodeURIComponent(key) + "=" + encodeURIComponent(v.value));
      } else {
        parts.push(encodeURIComponent(key));
      }
    }
    query = "?" + parts.join("&");
  }
  return {
    statusCode: 308,
    statusDescription: "Permanent Redirect",
    headers: {
      "location": { "value": "https://%s" + uri + query }
    }
  };
}`, strings.TrimSuffix(strings.TrimSpace(*props.ToDomainName), "."))

	redirect.Function = awscloudfront.NewFunction(construct, jsii.String("RedirectFunction"), &awscloudfront.FunctionProps{
		Code:    awscloudfront.FunctionCode_FromInline(jsii.String(code)),
		Runtime: awscloudfront.FunctionRuntime_JS_2_0(),
	})

	// Origin is unused (the function returns a response), but CloudFront requires one.
	origin := awscloudfrontorigins.NewHttpOrigin(props.ToDomainName, &awscloudfrontorigins.HttpOriginProps{
		ProtocolPolicy: awscloudfront.OriginProtocolPolicy_HTTPS_ONLY,
	})

	redirect.Distribution = awscloudfront.NewDistribution(construct, jsii.String("Distribution"), &awscloudfront.DistributionProps{
		DomainNames: &[]*string{props.FromDomainName},
		Certificate: cert,
		WebAclId:    props.WebAclId,
		EnableIpv6:  props.EnableIpv6,
		DefaultBehavior: &awscloudfront.BehaviorOptions{
			Origin:               origin,
			ViewerProtocolPolicy: awscloudfront.ViewerProtocolPolicy_REDIRECT_TO_HTTPS,
			CachePolicy:          awscloudfront.CachePolicy_CACHING_DISABLED(),
			FunctionAssociations: &[]*awscloudfront.FunctionAssociation{
				{
					Function:  redirect.Function,
					EventType: awscloudfront.FunctionEventType_VIEWER_REQUEST,
				},
			},
		},
	})

	if props.Tags != nil {
		for k, v := range *props.Tags {
			awscdk.Tags_Of(redirect.Distribution).Add(jsii.String(k), v, nil)
		}
	}
	awscdk.Tags_Of(redirect.Distribution).Add(jsii.String("Framework"), jsii.String("Lift"), nil)
	awscdk.Tags_Of(redirect.Distribution).Add(jsii.String("Component"), jsii.String("HostRedirect"), nil)

	aliasTarget := awsroute53.RecordTarget_FromAlias(awsroute53targets.NewCloudFrontTarget(redirect.Distribution))
	awsroute53.NewARecord(construct, jsii.String("AliasA"), &awsroute53.ARecordProps{
		Zone:       props.HostedZone,
		RecordName: relativeRecordName(props.HostedZone, props.FromDomainName),
		Target:     aliasTarget,
	})
	if props.EnableIpv6 != nil && *props.EnableIpv6 {
		awsroute53.NewAaaaRecord(construct, jsii.String("AliasAAAA"), &awsroute53.AaaaRecordProps{
			Zone:       props.HostedZone,
			RecordName: relativeRecordName(props.HostedZone, props.FromDomainName),
			Target:     aliasTarget,
		})
	}

	return redirect
}
