package web

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// adminTopList is one of the lists on the admin top list pages, such as the
// bins sorted by bytes. Each list has its own page and runs a single query,
// so that a slow query only slows down its own page.
type adminTopList struct {
	Sort  string
	Title string
	Empty string
	errno int
	query func(limit int) (any, error)
}

// adminTopListSection is a group of top lists that share a page, such as the
// top bins.
type adminTopListSection struct {
	path          string
	name          string
	template      string
	jsonKey       string
	errnoJSON     int
	errnoTemplate int
	lists         []adminTopList
}

// topListQuery adapts a DAO lookup to the query signature of adminTopList.
func topListQuery[T any](fn func(limit int) ([]T, error)) func(limit int) (any, error) {
	return func(limit int) (any, error) {
		return fn(limit)
	}
}

// sortPattern returns the route pattern that matches the sort keys of the
// section, so that unknown sort keys are not routed to the section.
func (s adminTopListSection) sortPattern() string {
	sorts := make([]string, 0, len(s.lists))
	for _, list := range s.lists {
		sorts = append(sorts, list.Sort)
	}
	return strings.Join(sorts, "|")
}

func (s adminTopListSection) list(sort string) (adminTopList, bool) {
	for _, list := range s.lists {
		if list.Sort == sort {
			return list, true
		}
	}
	return adminTopList{}, false
}

func (h *HTTP) adminTopListSections() []adminTopListSection {
	return []adminTopListSection{
		{
			path:          "/admin/bins",
			name:          "bins",
			template:      "admin_bins",
			jsonKey:       "bins",
			errnoJSON:     242,
			errnoTemplate: 243,
			lists: []adminTopList{
				{Sort: "updated", Title: "By last updated at", Empty: "No bins found.", errno: 237, query: topListQuery(h.dao.Bin().GetLastUpdated)},
				{Sort: "created", Title: "By created at", Empty: "No bins found.", errno: 241, query: topListQuery(h.dao.Bin().GetByCreated)},
				{Sort: "bytes", Title: "By bytes in total", Empty: "No bins found.", errno: 238, query: topListQuery(h.dao.Bin().GetByBytes)},
				{Sort: "files", Title: "By file count", Empty: "No bins found.", errno: 240, query: topListQuery(h.dao.Bin().GetByFiles)},
				{Sort: "downloads", Title: "By download count", Empty: "No bins found.", errno: 239, query: topListQuery(h.dao.Bin().GetByDownloads)},
			},
		},
		{
			path:          "/admin/clients",
			name:          "clients",
			template:      "admin_clients",
			jsonKey:       "clients",
			errnoJSON:     257,
			errnoTemplate: 258,
			lists: []adminTopList{
				{Sort: "last-active", Title: "By last active at", Empty: "No clients have been recorded yet.", errno: 249, query: topListQuery(h.dao.Client().GetByLastActiveAt)},
				{Sort: "requests", Title: "By number of requests", Empty: "No clients have been recorded yet.", errno: 250, query: topListQuery(h.dao.Client().GetByRequests)},
				{Sort: "banned", Title: "By banned at", Empty: "No clients are currently banned.", errno: 251, query: topListQuery(h.dao.Client().GetByBannedAt)},
				{Sort: "files-uploaded", Title: "By files uploaded", Empty: "No clients have uploaded files yet.", errno: 252, query: topListQuery(h.dao.Client().GetByFilesUploaded)},
				{Sort: "bytes-uploaded", Title: "By bytes uploaded", Empty: "No clients have uploaded files yet.", errno: 253, query: topListQuery(h.dao.Client().GetByBytesUploaded)},
				{Sort: "country", Title: "By country", Empty: "No countries recorded yet.", errno: 254, query: topListQuery(h.dao.Client().GetByCountry)},
				{Sort: "network", Title: "By network", Empty: "No networks recorded yet.", errno: 255, query: topListQuery(h.dao.Client().GetByNetwork)},
				{Sort: "asn", Title: "By ASN", Empty: "No ASNs recorded yet.", errno: 256, query: topListQuery(h.dao.Client().GetASNWithStats)},
			},
		},
		{
			path:          "/admin/files",
			name:          "files",
			template:      "admin_files",
			jsonKey:       "files",
			errnoJSON:     222,
			errnoTemplate: 223,
			lists: []adminTopList{
				{Sort: "created", Title: "By created at", Empty: "No files found.", errno: 494, query: topListQuery(h.dao.File().GetByCreated)},
				{Sort: "updated", Title: "By last updated at", Empty: "No files found.", errno: 495, query: topListQuery(h.dao.File().GetByUpdated)},
				{Sort: "bytes", Title: "By bytes", Empty: "No files found.", errno: 496, query: topListQuery(h.dao.File().GetByBytes)},
				{Sort: "downloads", Title: "By download count", Empty: "No files found.", errno: 497, query: topListQuery(h.dao.File().GetTopDownloads)},
				{Sort: "updates", Title: "By update count", Empty: "No files found.", errno: 498, query: topListQuery(h.dao.File().GetByUpdates)},
			},
		},
		{
			path:          "/admin/filecontent",
			name:          "file content entries",
			template:      "admin_filecontent",
			jsonKey:       "file_content",
			errnoJSON:     228,
			errnoTemplate: 229,
			lists: []adminTopList{
				{Sort: "references", Title: "By reference count", Empty: "No file content has been recorded yet.", errno: 493, query: topListQuery(h.dao.File().FilesByChecksum)},
				{Sort: "bytes-each", Title: "By bytes each", Empty: "No file content has been recorded yet.", errno: 224, query: topListQuery(h.dao.File().FilesByBytes)},
				{Sort: "bytes-total", Title: "By bytes total", Empty: "No file content has been recorded yet.", errno: 225, query: topListQuery(h.dao.File().FilesByBytesTotal)},
				{Sort: "created", Title: "By created at", Empty: "No file content has been recorded yet.", errno: 226, query: topListQuery(h.dao.FileContent().GetByCreated)},
				{Sort: "blocked", Title: "Blocked content", Empty: "No content is currently blocked.", errno: 227, query: topListQuery(h.dao.FileContent().GetBlocked)},
			},
		},
	}
}

// parseLimit returns the limit query parameter, or def if it is missing or
// outside 1–5000.
func parseLimit(r *http.Request, def int) int {
	if i, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && i >= 1 && i <= 5000 {
		return i
	}
	return def
}

// redirectAdminTopList redirects the section path to the first list of the
// section, keeping the query string, so that old links keep working.
func (h *HTTP) redirectAdminTopList(s adminTopListSection) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		target := s.path + "/" + s.lists[0].Sort
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusFound)
	}
}

func (h *HTTP) viewAdminTopList(s adminTopListSection) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		sort := mux.Vars(r)["sort"]
		list, found := s.list(sort)
		if !found {
			http.NotFound(w, r)
			return
		}
		limit := parseLimit(r, 25)

		items, err := list.query(limit)
		if err != nil {
			slog.Error("unable to get top list", "path", s.path, "sort", sort, "error", err)
			http.Error(w, fmt.Sprintf("Errno %d", list.errno), http.StatusInternalServerError)
			return
		}

		if r.Header.Get("accept") == "application/json" {
			w.Header().Set("Content-Type", "application/json")
			out, err := json.MarshalIndent(map[string]any{
				"sort":    sort,
				"limit":   limit,
				s.jsonKey: items,
			}, "", "    ")
			if err != nil {
				slog.Error("failed to parse json", "error", err)
				http.Error(w, fmt.Sprintf("Errno %d", s.errnoJSON), http.StatusInternalServerError)
				return
			}
			w.WriteHeader(200)
			_, _ = w.Write(out)
			return
		}

		type Data struct {
			Path  string
			Name  string
			Lists []adminTopList
			Sort  string
			Title string
			Empty string
			Limit int
			Items any
		}
		data := Data{
			Path:  s.path,
			Name:  s.name,
			Lists: s.lists,
			Sort:  sort,
			Title: list.Title,
			Empty: list.Empty,
			Limit: limit,
			Items: items,
		}
		if err := h.renderTemplate(w, s.template, data); err != nil {
			slog.Error("failed to execute template", "error", err)
			http.Error(w, fmt.Sprintf("Errno %d", s.errnoTemplate), http.StatusInternalServerError)
			return
		}
	}
}
