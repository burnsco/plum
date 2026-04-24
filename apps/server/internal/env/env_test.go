package env

import "testing"

func TestStringFallsBackForUnsetOrBlank(t *testing.T) {
	t.Setenv("PLUM_ENV_STRING_UNSET", "")
	if got := String("PLUM_ENV_STRING_UNSET", "/fallback"); got != "/fallback" {
		t.Fatalf("String blank = %q, want fallback", got)
	}

	if got := String("PLUM_ENV_STRING_MISSING", "/fallback"); got != "/fallback" {
		t.Fatalf("String missing = %q, want fallback", got)
	}
}

func TestStringReturnsTrimmedValue(t *testing.T) {
	t.Setenv("PLUM_ENV_STRING_VALUE", "  /tv  ")
	if got := String("PLUM_ENV_STRING_VALUE", "/fallback"); got != "/tv" {
		t.Fatalf("String value = %q, want /tv", got)
	}
}

func TestBoolRecognizesLegacyValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  bool
		ok    bool
	}{
		{name: "true", value: "true", want: true, ok: true},
		{name: "yes", value: "yes", want: true, ok: true},
		{name: "on", value: " on ", want: true, ok: true},
		{name: "false", value: "false", want: false, ok: true},
		{name: "no", value: "no", want: false, ok: true},
		{name: "off", value: " off ", want: false, ok: true},
		{name: "invalid", value: "maybe", want: false, ok: false},
		{name: "blank", value: "   ", want: false, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PLUM_ENV_BOOL_TEST", tt.value)
			got, ok := Bool("PLUM_ENV_BOOL_TEST")
			if got != tt.want || ok != tt.ok {
				t.Fatalf("Bool(%q) = (%v, %v), want (%v, %v)", tt.value, got, ok, tt.want, tt.ok)
			}
		})
	}
}
