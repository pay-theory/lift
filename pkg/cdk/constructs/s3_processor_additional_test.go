package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awslambda"
	"github.com/aws/aws-cdk-go/awscdk/v2/awss3"
	"github.com/aws/jsii-runtime-go"
)

func TestS3Processor_CrossRegionReplicationAndBucketPolicyHelpers(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	replicationBucket := awss3.NewBucket(stack, jsii.String("ReplicationBucket"), &awss3.BucketProps{
		BucketName: jsii.String("replication-bucket"),
	})

	processor := NewS3Processor(stack, jsii.String("Processor"), &S3ProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("replication-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		CrossRegionReplication: jsii.Bool(true),
		ReplicationBucket:      replicationBucket,
	})

	processor.EnableCORS([]*awss3.CorsRule{
		{
			AllowedMethods: &[]awss3.HttpMethods{awss3.HttpMethods_GET},
			AllowedOrigins: &[]*string{jsii.String("*")},
		},
	})

	processor.SetBucketPolicy(map[string]interface{}{
		"Statement": []interface{}{
			map[string]interface{}{
				"Effect":    "Allow",
				"Action":    "s3:GetObject",
				"Resource":  []interface{}{"arn:aws:s3:::example/*"},
				"Principal": map[string]interface{}{"AWS": "123456789012"},
			},
			map[string]interface{}{
				"Effect":    "Deny",
				"Action":    []interface{}{"s3:PutObject"},
				"Resource":  "arn:aws:s3:::example/*",
				"Principal": map[string]interface{}{"Service": "lambda.amazonaws.com"},
			},
		},
	})

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "AWS::S3::BucketPolicy")

	bucketResources := findResourcesByType(template, "AWS::S3::Bucket")
	for _, bucket := range bucketResources {
		props, ok := bucket["Properties"].(map[string]interface{})
		if !ok {
			continue
		}
		if _, ok := props["ReplicationConfiguration"]; ok {
			return
		}
	}

	t.Fatal("expected bucket replication configuration")
}

func TestS3Processor_ExternalBucketNotifications(t *testing.T) {
	app := awscdk.NewApp(nil)
	stack := awscdk.NewStack(app, jsii.String("TestStack"), nil)

	imported := awss3.Bucket_FromBucketName(stack, jsii.String("Imported"), jsii.String("imported-bucket"))
	external := awss3.NewBucket(stack, jsii.String("External"), &awss3.BucketProps{
		BucketName: jsii.String("external-bucket"),
	})

	processor := NewS3Processor(stack, jsii.String("Processor"), &S3ProcessorProps{
		FunctionProps: awslambda.FunctionProps{
			FunctionName: jsii.String("external-notify-processor"),
			Code:         awslambda.Code_FromInline(jsii.String("exports.handler = async () => {}")),
			Handler:      jsii.String("index.handler"),
			Runtime:      awslambda.Runtime_NODEJS_18_X(),
		},
		ExistingBucket: imported,
		ExternalBucket: external,
		EventFilter: &S3EventFilter{
			Prefix: jsii.String("images/"),
			Suffix: jsii.String(".png"),
		},
	})

	if processor.EventSource != nil {
		t.Fatal("expected no event source for external bucket path")
	}

	template := synthesizeTemplate(t, stack)
	assertResourceExists(t, template, "Custom::S3BucketNotifications")
}
