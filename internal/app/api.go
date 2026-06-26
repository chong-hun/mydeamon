package app

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/chenxian/learning-go-daemon/internal/tasks"
)

var errUnknownTaskAction = errors.New("unknown task action")

func newTaskHTTPHandler(service *tasks.Service, now func() time.Time) http.Handler {
	startedAt := now().UTC()
	mux := http.NewServeMux()

	mux.HandleFunc("/runtime", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{
			"status":     "running",
			"started_at": startedAt,
		})
	})

	mux.HandleFunc("/tasks", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			items, err := service.List()
			if err != nil {
				writeError(w, http.StatusInternalServerError, err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]any{"tasks": items})
		case http.MethodPost:
			var input tasks.CreateInput
			if err := decodeJSON(r, &input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			item, err := service.Create(input)
			if err != nil {
				writeError(w, taskErrorStatus(err), err)
				return
			}
			writeJSON(w, http.StatusCreated, item)
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	mux.HandleFunc("/tasks/", func(w http.ResponseWriter, r *http.Request) {
		id, action := splitTaskPath(r.URL.Path)
		if id == "" {
			http.NotFound(w, r)
			return
		}

		switch {
		case action == "" && r.Method == http.MethodGet:
			item, err := service.Get(id)
			if err != nil {
				writeError(w, taskErrorStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case action == "" && r.Method == http.MethodPatch:
			var input tasks.UpdateInput
			if err := decodeJSON(r, &input); err != nil {
				writeError(w, http.StatusBadRequest, err)
				return
			}

			item, err := service.Update(id, input)
			if err != nil {
				writeError(w, taskErrorStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, item)
		case action != "" && r.Method == http.MethodPost:
			if err := runTaskAction(service, id, action); err != nil {
				writeError(w, taskErrorStatus(err), err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})

	return mux
}

func splitTaskPath(path string) (string, string) {
	trimmed := strings.TrimPrefix(path, "/tasks/")
	parts := strings.Split(trimmed, "/")
	if len(parts) == 1 {
		return parts[0], ""
	}
	if len(parts) >= 2 {
		return parts[0], parts[1]
	}
	return "", ""
}

func runTaskAction(service *tasks.Service, id, action string) error {
	switch action {
	case "start":
		return service.Start(id)
	case "block":
		return service.Block(id)
	case "review":
		return service.Review(id)
	case "reopen":
		return service.Reopen(id)
	case "complete":
		return service.Complete(id)
	case "resume":
		return service.Resume(id)
	case "todo":
		return service.MoveToTodo(id)
	default:
		return errUnknownTaskAction
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func decodeJSON(r *http.Request, dst any) error {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder.Decode(dst)
}

func taskErrorStatus(err error) int {
	switch {
	case errors.Is(err, tasks.ErrValidation), errors.Is(err, errUnknownTaskAction):
		return http.StatusBadRequest
	case errors.Is(err, tasks.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(err, tasks.ErrInvalidTransition):
		return http.StatusConflict
	default:
		return http.StatusInternalServerError
	}
}
