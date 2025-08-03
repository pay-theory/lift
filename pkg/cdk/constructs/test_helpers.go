package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/stretchr/testify/assert"
)

// synthesizeTemplate synthesizes a CDK stack and returns the CloudFormation template
func synthesizeTemplate(stack awscdk.Stack) assertions.Template {
	return assertions.Template_FromStack(stack, nil)
}

// findResourcesByType finds all resources of a given type in the template
func findResourcesByType(template assertions.Template, resourceType string) map[string]map[string]interface{} {
	// FindResources expects a pointer to string
	resourceTypePtr := &resourceType
	resourcesPtr := template.FindResources(resourceTypePtr, nil)
	if resourcesPtr == nil {
		return make(map[string]map[string]interface{})
	}
	// Convert from map[string]*map[string]interface{} to map[string]map[string]interface{}
	result := make(map[string]map[string]interface{})
	for key, valuePtr := range *resourcesPtr {
		if valuePtr != nil {
			result[key] = *valuePtr
		}
	}
	return result
}

// assertResourceExists asserts that a resource of the given type exists in the template
func assertResourceExists(t *testing.T, template assertions.Template, resourceType string) {
	resources := findResourcesByType(template, resourceType)
	assert.Greater(t, len(resources), 0, "Expected at least one %s resource", resourceType)
}

// assertResourceCount asserts that exactly count resources of the given type exist
func assertResourceCount(t *testing.T, template assertions.Template, resourceType string, count int) {
	resources := findResourcesByType(template, resourceType)
	assert.Equal(t, count, len(resources), "Expected %d %s resources, got %d", count, resourceType, len(resources))
}