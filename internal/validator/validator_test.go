package validator

import "testing"

func TestValidatorValid(t *testing.T) {
	t.Parallel()

	v := New()
	if !v.Valid() {
		t.Fatal("new validator should be valid")
	}

	v.AddError("name", "must be provided")
	if v.Valid() {
		t.Fatal("validator with errors should not be valid")
	}
}

func TestValidatorKeepsFirstFieldError(t *testing.T) {
	t.Parallel()

	v := New()
	v.AddError("name", "first")
	v.AddError("name", "second")

	if got := v.Errors["name"]; got != "first" {
		t.Fatalf("got %q, want %q", got, "first")
	}
}

func TestStringHelpers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		got  bool
		want bool
	}{
		{
			name: "not blank rejects whitespace",
			got:  NotBlank("   "),
			want: false,
		},
		{
			name: "not blank accepts text",
			got:  NotBlank("latte"),
			want: true,
		},
		{
			name: "max chars counts runes",
			got:  MaxChars("кофе", 4),
			want: true,
		},
		{
			name: "min chars counts runes",
			got:  MinChars("чай", 4),
			want: false,
		},
		{
			name: "matches email",
			got:  Matches("user@example.com", EmailRX),
			want: true,
		},
		{
			name: "rejects invalid email",
			got:  Matches("not an email", EmailRX),
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if tt.got != tt.want {
				t.Fatalf("got %v, want %v", tt.got, tt.want)
			}
		})
	}
}

func TestPermittedValue(t *testing.T) {
	t.Parallel()

	if !PermittedValue("accepted", "pending", "accepted") {
		t.Fatal("expected value to be permitted")
	}
	if PermittedValue("cancelled", "pending", "accepted") {
		t.Fatal("expected value to be rejected")
	}
}

func TestUnique(t *testing.T) {
	t.Parallel()

	if !Unique([]string{"latte", "croissant"}) {
		t.Fatal("expected unique slice")
	}
	if Unique([]string{"latte", "latte"}) {
		t.Fatal("expected duplicate slice")
	}
}
