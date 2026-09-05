package zgw

import (
	"errors"
	"net/url"
	"strings"
)

// normalizeBaseURL macht aus einer Benutzereingabe wie "zgw16-ip.local"
// oder "http://192.168.1.50/" die vollstaendige API-Basisadresse
// "http://192.168.1.50/api".
func normalizeBaseURL(host string) (string, error) {
	host = strings.TrimSpace(host)
	if host == "" {
		return "", errors.New("keine Adresse angegeben")
	}
	if !strings.Contains(host, "://") {
		host = "http://" + host
	}
	u, err := url.Parse(host)
	if err != nil {
		return "", errors.New("Adresse nicht lesbar: " + host)
	}
	if u.Host == "" {
		return "", errors.New("Adresse enthaelt keinen Hostnamen: " + host)
	}
	p := strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(p, "/api") {
		p += "/api"
	}
	u.Path = p
	u.RawQuery = ""
	u.Fragment = ""
	return u.String(), nil
}
