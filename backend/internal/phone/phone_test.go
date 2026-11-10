package phone

import "testing"

func TestNormalize(t *testing.T) {
	ok := []string{
		"01712-345678", "8801712345678", "+880 1712 345678", "০১৭১২৩৪৫৬৭৮", "1712345678",
		"+880 01712 345678", "00880 1712 345678", "(+880) 1712-345678", "01712\u00a0345678", "01912.345.678",
		// v1.3: phone theke copy (lukano chinho, non-breaking hyphen), zero-width space, en dash
		"\u202a+880 1712\u2011345678\u202c", "01712\u200b345678", "০১৭১২–৩৪৫৬৭৮", "+ 880 1712-345678",
	}
	for _, in := range ok {
		got, err := Normalize(in)
		want := "+8801712345678"
		if in == "01912.345.678" {
			want = "+8801912345678"
		}
		if err != nil || got != want {
			t.Errorf("Normalize(%q) = %q, %v; want %q", in, got, err, want)
		}
	}
	bad := []string{
		"", "01212345678", "abc01712345678xyz", "০১৭১২-ABC-৩৪৫৬৭৮", "017123456789",
		"0171234567", "001712345678", "+1 212 555 0100", "+91 98765 43210",
		// v1.3: '+' er por 880 nai, '+' majhkhane, duita '+'
		"+1 712 345 678", "01712+345678", "++8801712345678",
	}
	for _, in := range bad {
		if got, err := Normalize(in); err != ErrInvalid {
			t.Errorf("Normalize(%q) = %q, %v; want ErrInvalid", in, got, err)
		}
	}
}
