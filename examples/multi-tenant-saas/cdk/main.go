package main

import (
	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/jsii-runtime-go"
	"github.com/pay-theory/lift/pkg/cdk/stacks"
)

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	stacks.NewMultiTenantSaaSStack(app, "MultiTenantSaaSStack", &stacks.MultiTenantSaaSStackProps{
		StackProps: awscdk.StackProps{
			Env: env(),
		},
		AppName:           "multi-tenant-saas-demo",
		CodePath:          "../dist/bootstrap",
		EnableAuth:        true,
		EnableFileStorage: true,
		// Uncomment and set these for custom domain
		// DomainName:     "api.example.com",
		// CertificateArn: "arn:aws:acm:us-east-1:...",
	})

	app.Synth(nil)
}

func env() *awscdk.Environment {
	// Customize this for your AWS environment
	// return &awscdk.Environment{
	//     Account: jsii.String("123456789012"),
	//     Region:  jsii.String("us-east-1"),
	// }
	return nil
}
