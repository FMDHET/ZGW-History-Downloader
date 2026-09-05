package export

import (
	"testing"
	"time"
)

func TestSafeName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Garage", "Garage"},
		{"Wohnung 3", "Wohnung 3"},
		{`A/B\C:D*E?F"G<H>I|J`, "A_B_C_D_E_F_G_H_I_J"},
		{"  Rand  ", "Rand"},
		{"Zeilen\numbruch", "Zeilen_umbruch"},
		{"", "unbenannt"},
		{"///", "unbenannt"},
	}
	for _, c := range cases {
		if got := SafeName(c.in); got != c.want {
			t.Errorf("SafeName(%q) = %q, erwartet %q", c.in, got, c.want)
		}
	}
}

func TestFileName(t *testing.T) {
	stamp := time.Date(2026, 9, 5, 8, 15, 30, 0, time.UTC)
	got := FileName("Garage", 14, stamp)
	want := "ZGW_Garage_14d_20260905-081530.csv"
	if got != want {
		t.Errorf("FileName = %q, erwartet %q", got, want)
	}
}
