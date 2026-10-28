package web

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"

	"github.com/gorilla/mux"

	"github.com/espebra/filebin2/internal/ds"
)

// ipParam returns the IP address in the request path in its canonical form,
// so that different spellings of the same IPv6 address refer to the same
// client. A port is accepted and dropped, since bans recorded the IP address
// of the admin with the port.
func ipParam(r *http.Request) (string, bool) {
	ip, err := extractIP(mux.Vars(r)["ip"])
	if err != nil {
		return "", false
	}
	return ip, true
}

func (h *HTTP) viewAdminIP(w http.ResponseWriter, r *http.Request) {
	ip, ok := ipParam(r)
	if !ok {
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	type Data struct {
		IP     string    `json:"ip"`
		Found  bool      `json:"found"`
		Client ds.Client `json:"client"`
		GeoIP  ds.Client `json:"geoip"`
		Own    bool      `json:"-"`
	}
	var data Data
	data.IP = ip
	data.Own = remoteIP(r) == ip

	client, found, err := h.dao.Client().GetByIP(net.ParseIP(ip))
	if err != nil {
		slog.Error("unable to get client by IP", "ip", ip, "error", err)
		http.Error(w, "Errno 277", http.StatusInternalServerError)
		return
	}
	data.Client = client
	data.Found = found

	// The client record lacks the geolocation if the client has not made
	// any requests since it was recorded, for instance when it was banned
	// before it was seen, so look it up from the GeoIP database as well.
	if h.geodb != nil {
		if err := h.geodb.Lookup(ip, &data.GeoIP); err != nil {
			slog.Debug("unable to look up geoip details", "ip", ip, "error", err)
		}
	}

	if r.Header.Get("accept") == "application/json" {
		w.Header().Set("Content-Type", "application/json")
		out, err := json.MarshalIndent(data, "", "    ")
		if err != nil {
			slog.Error("failed to parse json", "error", err)
			http.Error(w, "Errno 278", http.StatusInternalServerError)
			return
		}
		w.WriteHeader(200)
		_, _ = w.Write(out)
	} else {
		if err := h.renderTemplate(w, "admin_ip", data); err != nil {
			slog.Error("failed to execute template", "error", err)
			http.Error(w, "Errno 279", http.StatusInternalServerError)
			return
		}
	}
}

func (h *HTTP) banIP(w http.ResponseWriter, r *http.Request) {
	ip, ok := ipParam(r)
	if !ok {
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	if err := h.dao.Client().Ban([]string{ip}, remoteIP(r)); err != nil {
		slog.Error("unable to ban client", "ip", ip, "error", err)
		http.Error(w, "Errno 280", http.StatusInternalServerError)
		return
	}
	slog.Info("banned client", "ip", ip, "banned_by", remoteIP(r))

	http.Redirect(w, r, "/admin/ip/"+ip, http.StatusSeeOther)
}

func (h *HTTP) unbanIP(w http.ResponseWriter, r *http.Request) {
	ip, ok := ipParam(r)
	if !ok {
		http.Error(w, "Invalid IP address", http.StatusBadRequest)
		return
	}

	if err := h.dao.Client().Unban(ip); err != nil {
		slog.Error("unable to unban client", "ip", ip, "error", err)
		http.Error(w, "Errno 281", http.StatusInternalServerError)
		return
	}
	slog.Info("unbanned client", "ip", ip, "unbanned_by", remoteIP(r))

	http.Redirect(w, r, "/admin/ip/"+ip, http.StatusSeeOther)
}
