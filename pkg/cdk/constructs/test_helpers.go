package constructs

import (
	"testing"

	"github.com/aws/aws-cdk-go/awscdk/v2/assertions"
	"github.com/stretchr/testify/require"
)

// findResourcesByType finds all resources of a given type in the template
func findResourcesByType(template assertions.Template, resourceType string) map[string]map[string]interface{} {
	// Get template as map
	templateMap := template.ToJSON()
	if templateMap == nil {
		return nil
	}
	
	// Extract resources section
	resources, ok := (*templateMap)["Resources"].(map[string]interface{})
	if !ok {
		return nil
	}
	
	// Find matching resources
	result := make(map[string]map[string]interface{})
	for key, value := range resources {
		resource, ok := value.(map[string]interface{})
		if !ok {
			continue
		}
		
		if resType, ok := resource["Type"].(string); ok && resType == resourceType {
			result[key] = resource
		}
	}
	
	return result
}

// assertResourceCount asserts the count of resources of a given type
func assertResourceCount(t *testing.T, template assertions.Template, resourceType string, expectedCount int) {
	resources := findResourcesByType(template, resourceType)
	require.Equal(t, expectedCount, len(resources), "Expected %d resources of type %s, got %d", expectedCount, resourceType, len(resources))
}