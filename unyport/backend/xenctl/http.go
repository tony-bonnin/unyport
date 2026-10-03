package xenctl

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"

	"unyport/middleware"
)

type Handler struct {
	client *Client
	logger *slog.Logger
}

type actionRequest struct {
	Action     string `json:"action"`
	ConfigPath string `json:"config_path"`
	TargetHost string `json:"target_host"`
	Live       bool   `json:"live"`
	DryRun     bool   `json:"dry_run"`
}

func NewHandler(client *Client, logger *slog.Logger) *Handler {
	return &Handler{client: client, logger: logger}
}

func (h *Handler) Domains(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	domains, err := h.client.ListDomains(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]any{"domains": domains})
}

func (h *Handler) Info(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	info, err := h.client.Info(r.Context())
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadGateway)
		return
	}
	jsonOK(w, map[string]any{"info": info})
}

func (h *Handler) DomainAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	domain, ok := domainFromActionPath(r.URL.Path)
	if !ok {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	var req actionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}
	req.Action = strings.ToLower(strings.TrimSpace(req.Action))
	result, err := h.runAction(r, domain, req)
	if err != nil {
		h.logger.Warn("xen action refused", "domain", domain, "action", req.Action, "err", err)
		status := http.StatusBadRequest
		if err.Error() == "admin required" {
			status = http.StatusForbidden
		}
		jsonError(w, err.Error(), status)
		return
	}
	h.logger.Info("xen action", "user", middleware.UserEmailFromCtx(r.Context()), "role", middleware.UserRoleFromCtx(r.Context()), "domain", domain, "action", req.Action, "dry_run", req.DryRun)
	jsonOK(w, result)
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if middleware.UserRoleFromCtx(r.Context()) != "admin" {
		jsonError(w, "admin required", http.StatusForbidden)
		return
	}
	var req actionRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&req); err != nil {
		jsonError(w, "bad request", http.StatusBadRequest)
		return
	}
	result, err := h.client.Create(r.Context(), strings.TrimSpace(req.ConfigPath), req.DryRun)
	if err != nil {
		jsonError(w, err.Error(), http.StatusBadRequest)
		return
	}
	h.logger.Info("xen create", "user", middleware.UserEmailFromCtx(r.Context()), "config", req.ConfigPath, "dry_run", req.DryRun)
	jsonOK(w, result)
}

func (h *Handler) runAction(r *http.Request, domain string, req actionRequest) (ActionResult, error) {
	switch req.Action {
	case "shutdown":
		return h.client.Shutdown(r.Context(), domain, req.DryRun)
	case "reboot":
		return h.client.Reboot(r.Context(), domain, req.DryRun)
	case "pause":
		return h.client.Pause(r.Context(), domain, req.DryRun)
	case "unpause":
		return h.client.Unpause(r.Context(), domain, req.DryRun)
	case "destroy":
		if middleware.UserRoleFromCtx(r.Context()) != "admin" {
			return ActionResult{}, errAdminRequired()
		}
		return h.client.Destroy(r.Context(), domain, req.DryRun)
	case "migrate":
		if middleware.UserRoleFromCtx(r.Context()) != "admin" {
			return ActionResult{}, errAdminRequired()
		}
		return h.client.Migrate(r.Context(), domain, strings.TrimSpace(req.TargetHost), req.Live, req.DryRun)
	default:
		return ActionResult{}, errUnknownAction()
	}
}

func domainFromActionPath(path string) (string, bool) {
	const prefix = "/api/xen/domains/"
	const suffix = "/actions"
	if !strings.HasPrefix(path, prefix) || !strings.HasSuffix(path, suffix) {
		return "", false
	}
	domain := strings.TrimSuffix(strings.TrimPrefix(path, prefix), suffix)
	domain = strings.Trim(domain, "/")
	return domain, domain != ""
}

func jsonOK(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]any{"error": msg})
}

type actionError string

func (e actionError) Error() string { return string(e) }

func errAdminRequired() error { return actionError("admin required") }
func errUnknownAction() error { return actionError("unknown action") }
