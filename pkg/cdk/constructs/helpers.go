package constructs

// intPtr returns a pointer to an int value
// This is a helper function for CDK properties that require pointer types
func intPtr(i int) *int {
	return &i
}
