package constructs

import (
	"strings"
	"testing"
)

func TestGenerateCRUDHandlerCode_ReturnsExpectedHandlers(t *testing.T) {
	cases := []struct {
		operation string
		contains  string
	}{
		{operation: "create", contains: "CREATE Event"},
		{operation: "read", contains: "READ Event"},
		{operation: "update", contains: "UPDATE Event"},
		{operation: "delete", contains: "DELETE Event"},
		{operation: "list", contains: "LIST Event"},
		{operation: "search", contains: "SEARCH Event"},
		{operation: "unknown", contains: "Not implemented"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.operation, func(t *testing.T) {
			code := GenerateCRUDHandlerCode(tc.operation)
			if !strings.Contains(code, tc.contains) {
				t.Fatalf("expected handler code for %q to contain %q", tc.operation, tc.contains)
			}
		})
	}
}
