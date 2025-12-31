package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestLiftHostedZone_CreatesZoneAndRecords(t *testing.T) {
	stack := test.NewTestStack()

	zone := NewLiftHostedZone(stack.Stack(), jsii.String("Zone"), &LiftHostedZoneProps{
		ZoneName: jsii.String("example.com"),
		Comment:  jsii.String("test zone"),
		Tags: &map[string]*string{
			"Environment": jsii.String("test"),
		},
	})

	if zone.IsImported {
		t.Fatal("expected zone to be created, not imported")
	}
	if zone.GetNameServers() == nil {
		t.Fatal("expected name servers for created zone")
	}

	zone.AddCNAMERecord(jsii.String("www"), jsii.String("target.example.com"), awscdk.Duration_Minutes(jsii.Number(5)))
	zone.AddNSRecord(jsii.String("sub"), &[]*string{jsii.String("ns-1.example.com"), jsii.String("ns-2.example.com")}, awscdk.Duration_Minutes(jsii.Number(5)))

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::Route53::HostedZone"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::Route53::RecordSet"), jsii.Number(2))
}

func TestLiftHostedZone_ImportsZoneAndExportsId(t *testing.T) {
	stack := test.NewTestStack()

	zone := NewLiftHostedZone(stack.Stack(), jsii.String("Zone"), &LiftHostedZoneProps{
		ZoneName:        jsii.String("example.com"),
		ImportIfExists:  jsii.Bool(true),
		ExistingZoneId:  jsii.String("Z1234567890"),
		EnableSSMExport: jsii.Bool(true),
		EnableCfnExport: jsii.Bool(true),
	})

	if !zone.IsImported {
		t.Fatal("expected zone to be imported")
	}
	if zone.GetNameServers() != nil {
		t.Fatal("expected no name servers for imported zone")
	}

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::Route53::HostedZone"), jsii.Number(0))
	template.HasResourceProperties(jsii.String("AWS::SSM::Parameter"), map[string]interface{}{
		"Name": "/route53/zones/example.com/id",
	})

	templateJSON := template.ToJSON()
	outputs, ok := (*templateJSON)["Outputs"].(map[string]interface{})
	if !ok || outputs == nil {
		t.Fatal("expected Outputs in synthesized template")
	}

	found := false
	for _, raw := range outputs {
		out, ok := raw.(map[string]interface{})
		if !ok || out == nil {
			continue
		}

		export, ok := out["Export"].(map[string]interface{})
		if !ok || export == nil {
			continue
		}

		if export["Name"] == "HostedZoneId-example-com" {
			found = true
			break
		}
	}

	if !found {
		t.Fatal("expected an output exported as HostedZoneId-example-com")
	}
}
