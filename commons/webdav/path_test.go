package webdav

import (
	"net/url"
	"testing"
)

func TestGetPathForTicketEscapesPathAndQuery(t *testing.T) {
	client := &WebDAVClient{baseURL: "https://example.org/dav"}
	webdavPath := client.getPathForTicket("/zone/a file#1&2", "ticket&value")
	parsedURL, err := url.Parse(webdavPath)
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if parsedURL.EscapedPath() != "/dav/zone/a%20file%231&2" {
		t.Errorf("EscapedPath() = %q", parsedURL.EscapedPath())
	}
	if parsedURL.Query().Get("ticket") != "ticket&value" {
		t.Errorf("ticket = %q, want ticket&value", parsedURL.Query().Get("ticket"))
	}
}
