package web

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func adminPost(h *HTTP, path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, nil)
	req.Header.Set("Authorization", basicAuth(testAdminUser, testAdminPass))
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr
}

func uploadFrom(h *HTTP, ip, path string) int {
	req := uploadRequest(path, "upload from "+ip)
	req.RemoteAddr = ip + ":1234"
	rr := httptest.NewRecorder()
	h.router.ServeHTTP(rr, req)
	return rr.Code
}

func TestAdminIPBanUnban(t *testing.T) {
	h := setupActionsHandler(t, nil)

	ip := "198.51.100.7"

	// An IP address that has not been seen has no client record
	rr := adminGet(h, "/admin/ip/"+ip, "")
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected the IP page to give %d, got %d. Body: %s", http.StatusOK, rr.Code, rr.Body.String())
	}
	if !strings.Contains(rr.Body.String(), "Not banned") || !strings.Contains(rr.Body.String(), "Not seen recently") {
		t.Errorf("Expected the IP page to show an unseen IP address that is not banned")
	}

	// Ban the IP address
	rr = adminPost(h, "/admin/ip/"+ip+"/ban")
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected ban to give %d, got %d. Body: %s", http.StatusSeeOther, rr.Code, rr.Body.String())
	}
	if got, want := rr.Header().Get("Location"), "/admin/ip/"+ip; got != want {
		t.Errorf("Expected ban to redirect to %s, got %s", want, got)
	}
	rr = adminGet(h, "/admin/ip/"+ip, "")
	if !strings.Contains(rr.Body.String(), "Banned") || !strings.Contains(rr.Body.String(), "/admin/ip/"+ip+"/unban") {
		t.Errorf("Expected the IP page to show the IP address as banned with an unban action")
	}
	if code := uploadFrom(h, ip, "/ipbanbin1234/file.txt"); code != http.StatusForbidden {
		t.Errorf("Expected upload from banned IP address to give %d, got %d", http.StatusForbidden, code)
	}

	// The JSON output shows the ban
	rr = adminGet(h, "/admin/ip/"+ip, "application/json")
	var out struct {
		IP     string `json:"ip"`
		Found  bool   `json:"found"`
		Client struct {
			BannedBy string `json:"banned_by"`
		} `json:"client"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &out); err != nil {
		t.Fatalf("Expected valid JSON from the IP page: %v", err)
	}
	if out.IP != ip || !out.Found || out.Client.BannedBy != "192.0.2.1" {
		t.Errorf("Expected the JSON to show %s as banned by 192.0.2.1, got %+v", ip, out)
	}

	// Unban the IP address
	rr = adminPost(h, "/admin/ip/"+ip+"/unban")
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected unban to give %d, got %d. Body: %s", http.StatusSeeOther, rr.Code, rr.Body.String())
	}
	rr = adminGet(h, "/admin/ip/"+ip, "")
	if !strings.Contains(rr.Body.String(), "Not banned") || !strings.Contains(rr.Body.String(), "/admin/ip/"+ip+"/ban") {
		t.Errorf("Expected the IP page to show the IP address as not banned with a ban action")
	}
	if code := uploadFrom(h, ip, "/ipbanbin1234/file.txt"); code != http.StatusCreated {
		t.Errorf("Expected upload from unbanned IP address to give %d, got %d", http.StatusCreated, code)
	}

	// The client record is shown after the IP address has made requests
	rr = adminGet(h, "/admin/ip/"+ip, "")
	if !strings.Contains(rr.Body.String(), "Requests") {
		t.Errorf("Expected the IP page to show the client record")
	}

	// Ban and unban require admin credentials
	for _, path := range []string{"/admin/ip/" + ip, "/admin/ip/" + ip + "/ban", "/admin/ip/" + ip + "/unban"} {
		method := http.MethodPost
		if path == "/admin/ip/"+ip {
			method = http.MethodGet
		}
		req := httptest.NewRequest(method, path, nil)
		rr := httptest.NewRecorder()
		h.router.ServeHTTP(rr, req)
		if rr.Code != http.StatusUnauthorized {
			t.Errorf("Expected unauthenticated %s %s to give %d, got %d", method, path, http.StatusUnauthorized, rr.Code)
		}
	}
}

func TestAdminIPAddressForms(t *testing.T) {
	h := setupActionsHandler(t, nil)

	// Invalid IP addresses are rejected
	if rr := adminGet(h, "/admin/ip/not-an-ip", ""); rr.Code != http.StatusBadRequest {
		t.Errorf("Expected an invalid IP address to give %d, got %d", http.StatusBadRequest, rr.Code)
	}
	if rr := adminPost(h, "/admin/ip/not-an-ip/ban"); rr.Code != http.StatusBadRequest {
		t.Errorf("Expected a ban of an invalid IP address to give %d, got %d", http.StatusBadRequest, rr.Code)
	}

	// IPv6 addresses are stored in their canonical form
	rr := adminPost(h, "/admin/ip/2001:0db8:0:0::1/ban")
	if rr.Code != http.StatusSeeOther {
		t.Fatalf("Expected ban to give %d, got %d. Body: %s", http.StatusSeeOther, rr.Code, rr.Body.String())
	}
	if got, want := rr.Header().Get("Location"), "/admin/ip/2001:db8::1"; got != want {
		t.Errorf("Expected ban to redirect to %s, got %s", want, got)
	}
	client, found, err := h.dao.Client().GetByIP(net.ParseIP("2001:db8::1"))
	if err != nil {
		t.Fatal(err)
	}
	if !found || !client.IsBanned() {
		t.Errorf("Expected 2001:db8::1 to be banned")
	}

	// An IP address with a port, as recorded by older bans, shows the IP address
	rr = adminGet(h, "/admin/ip/198.51.100.8:1234", "")
	if rr.Code != http.StatusOK {
		t.Fatalf("Expected an IP address with a port to give %d, got %d", http.StatusOK, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "<code>198.51.100.8</code>") {
		t.Errorf("Expected the IP page to show the IP address without the port")
	}

	// The admin is warned when looking at their own IP address
	if rr := adminGet(h, "/admin/ip/192.0.2.1", ""); !strings.Contains(rr.Body.String(), "This is your own IP address") {
		t.Errorf("Expected a warning on the page for the admin's own IP address")
	}
	if rr := adminGet(h, "/admin/ip/198.51.100.8", ""); strings.Contains(rr.Body.String(), "This is your own IP address") {
		t.Errorf("Expected no warning on the page for another IP address")
	}
}
