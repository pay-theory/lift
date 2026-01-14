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

func TestValidationErrors_ErrorEmpty(t *testing.T) {
	var errs ValidationErrors
	if errs.Error() != "validation failed" {
		t.Fatalf("expected empty ValidationErrors to return generic message, got %q", errs.Error())
	}
}

func TestValidate_IgnoresNonStructInputs(t *testing.T) {
	type payload struct {
		secret string `validate:"required"`
	}

	if err := Validate(payload{}); err != nil {
		t.Fatalf("expected unexported fields to be ignored, got %v", err)
	}
	if err := Validate("not a struct"); err != nil {
		t.Fatalf("expected non-struct inputs to be ignored, got %v", err)
	}

	var p *payload
	if err := Validate(p); err != nil {
		t.Fatalf("expected nil pointers to be ignored, got %v", err)
	}

	s := "ok"
	if err := Validate(&s); err != nil {
		t.Fatalf("expected pointer-to-non-struct inputs to be ignored, got %v", err)
	}
}

func TestValidate_AdditionalKindsAndUnknownRules(t *testing.T) {
	type payload struct {
		Count      uint              `validate:"required"`
		UintMin    uint              `validate:"min=1"`
		Ratio      float64           `validate:"min=2"`
		Enabled    bool              `validate:"required"`
		Items      []string          `validate:"required"`
		Config     map[string]string `validate:"required"`
		Any        any               `validate:"required"`
		Maybe      string            `validate:"oneof=admin user"`
		EmailInt   int               `validate:"email"`
		OneOfInt   int               `validate:"oneof=a b"`
		Unknown    string            `validate:"nonsense"`
		NotTagged  string
		Trailing   string `validate:"required,"`
		EmptyArray [0]int `validate:"required"`
	}

	err := Validate(payload{
		Count:      0,
		UintMin:    0,
		Ratio:      1.0,
		Enabled:    false,
		Items:      nil,
		Config:     map[string]string{},
		Any:        nil,
		Maybe:      "",
		EmailInt:   123,
		OneOfInt:   1,
		Unknown:    "ok",
		NotTagged:  "",
		Trailing:   "",
		EmptyArray: [0]int{},
	})
	if err == nil {
		t.Fatalf("expected validation errors")
	}

	var validationErrs ValidationErrors
	if !errors.As(err, &validationErrs) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}

	seen := make(map[string]bool, len(validationErrs))
	for _, vErr := range validationErrs {
		seen[vErr.Field] = true
	}

	for _, field := range []string{"Count", "Ratio", "Enabled", "Items", "Config", "Any", "Trailing", "EmptyArray"} {
		if !seen[field] {
			t.Errorf("expected error for field %s", field)
		}
	}
	for _, field := range []string{"UintMin", "Maybe", "EmailInt", "OneOfInt", "Unknown", "NotTagged"} {
		if seen[field] {
			t.Errorf("expected no error for field %s", field)
		}
	}
}

func TestValidateStruct_PrefixAndInvalidRuleValue(t *testing.T) {
	type payload struct {
		Name string `validate:"min=bogus"`
	}

	err := validateStruct(payload{Name: "x"}, "parent")
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErrs ValidationErrors
	if !errors.As(err, &validationErrs) {
		t.Fatalf("expected ValidationErrors, got %T", err)
	}
	if len(validationErrs) != 1 {
		t.Fatalf("expected 1 validation error, got %d", len(validationErrs))
	}

	if validationErrs[0].Field != "parent.Name" {
		t.Fatalf("expected prefixed field name, got %q", validationErrs[0].Field)
	}
	if !strings.Contains(validationErrs[0].Message, "invalid min rule value") {
		t.Fatalf("expected invalid rule value message, got %q", validationErrs[0].Message)
	}
}
