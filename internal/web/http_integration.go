package web

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *HTTP) integrationSlack(w http.ResponseWriter, r *http.Request) {
	// Interpret empty secret as not enabled, so reject early in this case
	if h.config.SlackSecret == "" {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Information needed for auth
	ts := r.Header.Get("X-Slack-Request-Timestamp")
	sig := r.Header.Get("X-Slack-Signature")
	body, err := io.ReadAll(r.Body)
	defer func() { _ = r.Body.Close() }()
	if err != nil {
		h.Error(w, r, fmt.Sprintf("Failed to read request body: %s\n", err.Error()), "Internal Server Error", 833, http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(body))

	// Reject old signatures. Allowing 60 seconds splay.
	ts_int, err := strconv.ParseInt(ts, 10, 64)
	if err != nil {
		http.Error(w, "Timestamp must be int", http.StatusBadRequest)
		return
	}
	now := time.Now()
	epoch := now.Unix()
	if epoch-60 > ts_int {
		h.Error(w, r, fmt.Sprintf("Replay rejected. Got timestamp: %q, actual timestamp: %d\n", ts, epoch), "Replay rejected", 834, http.StatusBadRequest)
		return
	}

	slog.Debug("slack request", "timestamp", ts, "epoch", epoch, "signature", sig)
	slog.Debug("slack request body", "body", string(body))

	// Generate signature and compare
	hash := hmac.New(sha256.New, []byte(h.config.SlackSecret))
	sig_basestring := fmt.Sprintf("v0:%s:%s", ts, body)
	hash.Write([]byte(sig_basestring))
	hash_signature := hex.EncodeToString(hash.Sum(nil))
	slog.Debug("generated hash signature", "signature", hash_signature)
	if !hmac.Equal([]byte(sig), []byte(fmt.Sprintf("v0=%s", hash_signature))) {
		h.Error(w, r, fmt.Sprintf("Slack signature not correct: Got %q, generated %q\n", sig, hash_signature), "Unauthorized", 835, http.StatusUnauthorized)
		return
	}

	domain := r.PostFormValue("team_domain")
	if h.config.SlackDomain != domain {
		h.Error(w, r, fmt.Sprintf("Slack domain not correct: Got %q, requires %q\n", domain, h.config.SlackDomain), "Unauthorized", 835, http.StatusUnauthorized)
		return
	}

	channel := r.PostFormValue("channel_name")
	if h.config.SlackChannel != channel {
		h.Error(w, r, fmt.Sprintf("Slack channel not correct: Got %q, requires %q\n", channel, h.config.SlackChannel), "Unauthorized", 835, http.StatusUnauthorized)
		return
	}

	// Handle commands
	command := r.PostFormValue("command")
	text := r.PostFormValue("text")
	slog.Debug("slack command", "command", command, "text", text)

	if command != "/filebin" {
		h.Error(w, r, fmt.Sprintf("Unknown command, got: %q\n", command), "Bad Request", 836, http.StatusBadRequest)
		return
	}

	s := strings.Fields(text)
	if len(s) > 0 {
		if s[0] == "approve" {
			if len(s) == 2 {
				inputBin := s[1]
				bin, found, err := h.dao.Bin().GetByID(inputBin)
				if err != nil {
					slog.Error("unable to get bin by ID", "bin", bin.Id, "error", err)
					http.Error(w, "Errno 205", http.StatusInternalServerError)
					return
				}
				if !found {
					http.Error(w, "Bin does not exist", http.StatusNotFound)
					return
				}

				if !bin.IsReadable() {
					http.Error(w, "This bin is no longer available", http.StatusNotFound)
					return
				}

				// No need to set the bin to approved twice
				if bin.IsApproved() {
					http.Error(w, "This bin is already approved", http.StatusOK)
					return
				}

				// Set bin as approved with the current timestamp
				now := time.Now().UTC().Truncate(time.Microsecond)
				_ = bin.ApprovedAt.Scan(now)
				if err := h.dao.Bin().Update(&bin); err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					return
				}

				http.Error(w, "Bin approved successfully.", http.StatusOK)
				return
			}
		}
		if s[0] == "lastupdated" {
			limit := 10
			if len(s) == 2 {
				limit, err = strconv.Atoi(s[1])
				if err != nil {
					http.Error(w, "/filebin lastupdated limit (limit must be int)", http.StatusOK)
					return
				}
				if limit <= 0 {
					limit = 10
				}
				if limit > 100 {
					limit = 100
				}
			}
			bins, err := h.dao.Bin().GetLastUpdated(limit)
			if err != nil {
				slog.Error("unable to get last updated", "error", err)
				http.Error(w, "Errno 8214", http.StatusInternalServerError)
				return
			}

			_, _ = io.WriteString(w, fmt.Sprintf("%d last updated bins:\n", limit))
			for _, bin := range bins {
				out := fmt.Sprintf("%s: %s/%s", bin.UpdatedAtRelative, h.config.BaseUrl.String(), bin.Id)
				if bin.IsApproved() {
					out = fmt.Sprintf("%s (approved)", out)
				} else {
					out = fmt.Sprintf("%s (pending)", out)
				}
				out = fmt.Sprintf("%s\n", out)
				_, _ = io.WriteString(w, out)
			}
			return
		}
	}

	// Print help for any /filebin commands we don't recognize
	h.slackHelpText(w, r)
}

func (h *HTTP) slackHelpText(w http.ResponseWriter, r *http.Request) {
	_, _ = io.WriteString(w, "Help for filebin Slack integration\n\n")
	_, _ = io.WriteString(w, "Approve bin [string]:\n")
	_, _ = io.WriteString(w, "  /filebin approve bin\n\n")
	_, _ = io.WriteString(w, "Print the 10 last updated bins:\n")
	_, _ = io.WriteString(w, "  /filebin lastupdated\n\n")
	_, _ = io.WriteString(w, "Print the n [int] last updated bins. Limited to 100:\n")
	_, _ = io.WriteString(w, "  /filebin lastupdated n\n")
}
