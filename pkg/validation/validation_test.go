package validation

import (
	"errors"
	"strings"
	"testing"
)

func TestValidate(t *testing.T) {
	type address struct {
		Street string `validate:"min=5,max=50"`
		Zip    string `validate:"max=5"`
	}

	type user struct {
		Name          string   `validate:"required,min=3,max=20"`
		Age           int      `validate:"min=18,max=65"`
		Role          string   `validate:"required,oneof=admin user"`
		PrimaryEmail  string   `validate:"required,email"`
		BackupEmail   string   `validate:"email"`
		OptionalEmail string   `validate:"omitempty,email"`
		Code          string   `validate:"max=4"`
		Location      *address `validate:"required"`
	}

	valid := &user{
		Name:          "Alex Doe",
		Age:           30,
		Role:          "admin",
		PrimaryEmail:  "alex@example.com",
		BackupEmail:   "backup@example.com",
		OptionalEmail: "",
		Code:          "1234",
		Location: &address{
			Street: "12345 Main Street",
			Zip:    "54321",
		},
	}

	if err := Validate(valid); err != nil {
		t.Fatalf("expected no error for valid input, got %v", err)
	}

	invalid := &user{
		Name:          "",
		Age:           16,
		Role:          "guest",
		PrimaryEmail:  "not-an-email",
		BackupEmail:   "invalid-email",
		OptionalEmail: "",
		Code:          "toolong",
		Location:      nil,
	}

	err := Validate(invalid)
	if err == nil {
		t.Fatalf("expected error for invalid input")
	}

	var validationErrs ValidationErrors
	if !errors.As(err, &validationErrs) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	expected := map[string][]string{
		"Name": {
			"field is required",
			"field must be at least 3 characters",
		},
		"Age": {
			"field must be at least 18",
		},
		"Role": {
			"field must be one of: admin, user",
		},
		"PrimaryEmail": {
			"field must be a valid email address",
		},
		"BackupEmail": {
			"field must be a valid email address",
		},
		"Code": {
			"field must be at most 4 characters",
		},
		"Location": {
			"field is required",
		},
	}

	seen := make(map[string][]string)
	for _, vErr := range validationErrs {
		seen[vErr.Field] = append(seen[vErr.Field], vErr.Message)
	}

	for field, wantMessages := range expected {
		gotMessages := seen[field]
		if len(gotMessages) != len(wantMessages) {
			t.Fatalf("field %s: expected %d errors, got %d (messages=%v)", field, len(wantMessages), len(gotMessages), gotMessages)
		}
		for _, want := range wantMessages {
			found := false
			for _, got := range gotMessages {
				if got == want {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("field %s: expected error message %q, got %v", field, want, gotMessages)
			}
		}
	}

	if _, ok := seen["OptionalEmail"]; ok {
		t.Errorf("optional field should not produce validation errors, got %v", seen["OptionalEmail"])
	}

	if count := strings.Count(err.Error(), "; "); count != len(validationErrs)-1 {
		t.Errorf("expected aggregated error string to contain %d separators, got %d", len(validationErrs)-1, count)
	}
}
