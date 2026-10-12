package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/autobrr/qui/internal/models"
	"github.com/autobrr/qui/internal/services/scripttasks"
)

type ScriptTasksHandler struct{ service *scripttasks.Service }

func NewScriptTasksHandler(service *scripttasks.Service) *ScriptTasksHandler {
	return &ScriptTasksHandler{service}
}
func (h *ScriptTasksHandler) Routes(r chi.Router) {
	r.Get("/", h.List)
	r.Post("/scripts", h.SaveScript)
	r.Delete("/scripts/{id}", h.DeleteScript)
	r.Post("/tasks", h.SaveTask)
	r.Delete("/tasks/{id}", h.DeleteTask)
	r.Post("/tasks/{id}/run", h.Run)
	r.Post("/tasks/{id}/cancel", h.Cancel)
	r.Get("/tasks/{id}/runs", h.Runs)
}
func scriptReply(w http.ResponseWriter, value any, err error) {
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}
func (h *ScriptTasksHandler) List(w http.ResponseWriter, r *http.Request) {
	scripts, err := h.service.Store.Scripts(r.Context())
	if err != nil {
		scriptReply(w, nil, err)
		return
	}
	tasks, err := h.service.Store.Tasks(r.Context())
	if err != nil {
		scriptReply(w, nil, err)
		return
	}
	_, runtimeErr := h.service.Runtime()
	scriptReply(w, struct {
		Scripts         []models.ScriptDefinition `json:"scripts"`
		Tasks           []models.ScriptTask       `json:"tasks"`
		PythonAvailable bool                      `json:"python_available"`
	}{scripts, tasks, runtimeErr == nil}, nil)
}
func (h *ScriptTasksHandler) SaveScript(w http.ResponseWriter, r *http.Request) {
	var item models.ScriptDefinition
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&item); err != nil {
		scriptReply(w, nil, err)
		return
	}
	item, err := h.service.SaveScript(r.Context(), item)
	scriptReply(w, item, err)
}
func (h *ScriptTasksHandler) DeleteScript(w http.ResponseWriter, r *http.Request) {
	scriptReply(w, struct{}{}, h.service.DeleteScript(r.Context(), chi.URLParam(r, "id")))
}
func (h *ScriptTasksHandler) SaveTask(w http.ResponseWriter, r *http.Request) {
	var item models.ScriptTask
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(&item); err != nil {
		scriptReply(w, nil, err)
		return
	}
	item, err := h.service.SaveTask(r.Context(), item)
	scriptReply(w, item, err)
}
func (h *ScriptTasksHandler) DeleteTask(w http.ResponseWriter, r *http.Request) {
	scriptReply(w, struct{}{}, h.service.DeleteTask(r.Context(), chi.URLParam(r, "id")))
}
func (h *ScriptTasksHandler) Run(w http.ResponseWriter, r *http.Request) {
	var input struct {
		DryRun bool `json:"dry_run"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&input); err != nil {
		scriptReply(w, nil, err)
		return
	}
	run, err := h.service.Run(chi.URLParam(r, "id"), input.DryRun)
	scriptReply(w, run, err)
}
func (h *ScriptTasksHandler) Cancel(w http.ResponseWriter, r *http.Request) {
	scriptReply(w, struct {
		Canceled bool `json:"canceled"`
	}{h.service.Cancel(chi.URLParam(r, "id"))}, nil)
}
func (h *ScriptTasksHandler) Runs(w http.ResponseWriter, r *http.Request) {
	runs, err := h.service.Store.Runs(r.Context(), chi.URLParam(r, "id"))
	scriptReply(w, runs, err)
}
