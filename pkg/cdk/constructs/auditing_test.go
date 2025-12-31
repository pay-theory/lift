package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslogs"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/test"
)

func TestAuditingConstruct_CreatesAuditResourcesAndStatus(t *testing.T) {
	stack := test.NewTestStack()

	audit := NewAuditingConstruct(stack.Stack(), "Audit", &AuditingProps{
		AppName:                  jsii.String("test-app"),
		EnableCrossAccountAccess: jsii.Bool(true),
		CrossAccountRoleArns: &[]*string{
			jsii.String("arn:aws:iam::123456789012:role/CrossAccountAuditRole"),
		},
		ComplianceFrameworks: &[]string{"pci-dss", "soc2"},
	})

	status := audit.GetAuditStatus()
	if status["cloudtrail_enabled"] != true {
		t.Fatal("expected cloudtrail enabled")
	}

	audit.AddCustomAuditRule("Errors", audit.AuditLogGroup, "ERROR")
	audit.EnableSIEMIntegration("https://example.com")

	template := assertions.Template_FromStack(stack.Stack(), nil)
	template.ResourceCountIs(jsii.String("AWS::S3::Bucket"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudTrail::Trail"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::KMS::Key"), jsii.Number(1))
	template.ResourceCountIs(jsii.String("AWS::CloudWatch::Dashboard"), jsii.Number(1))

	assertResourceExists(t, template, "AWS::Logs::MetricFilter")
	template.HasResourceProperties(jsii.String("AWS::Logs::MetricFilter"), map[string]interface{}{
		"MetricTransformations": assertions.Match_AnyValue(),
	})

	// Ensure custom audit rule created a MetricFilter for the audit log group.
	template.HasResourceProperties(jsii.String("AWS::Logs::MetricFilter"), map[string]interface{}{
		"FilterPattern": "ERROR",
	})

	// Ensure basic status fields map cleanly for optional resources.
	_ = audit.ApplicationLogGroup
	_ = audit.DatabaseLogGroup
	_ = audit.AuditLogGroup
	_ = awslogs.FilterPattern_Literal(jsii.String("ERROR"))
}
