package zgw

import "testing"

func TestNormalizeBaseURL(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"zgw16-ip.local", "http://zgw16-ip.local/api"},
		{"192.168.1.50", "http://192.168.1.50/api"},
		{"http://192.168.1.50", "http://192.168.1.50/api"},
		{"http://192.168.1.50/", "http://192.168.1.50/api"},
		{"http://192.168.1.50/api", "http://192.168.1.50/api"},
		{"https://zgw16-ip.local/api/", "https://zgw16-ip.local/api"},
		{"  zgw16-ip.local  ", "http://zgw16-ip.local/api"},
	}
	for _, c := range cases {
		got, err := normalizeBaseURL(c.in)
		if err != nil {
			t.Errorf("normalizeBaseURL(%q): unerwarteter Fehler %v", c.in, err)
			continue
		}
		if got != c.want {
			t.Errorf("normalizeBaseURL(%q) = %q, erwartet %q", c.in, got, c.want)
		}
	}
}

func TestNormalizeBaseURLRejectsEmpty(t *testing.T) {
	if _, err := normalizeBaseURL("   "); err == nil {
		t.Fatal("leere Adresse muss einen Fehler ergeben")
	}
}
