package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"

	"nhooyr.io/websocket"
	"nhooyr.io/websocket/wsjson"
)

//go:embed static
var staticFiles embed.FS

type AgentNode struct {
	ID             string `json:"id"`
	ParentID       string `json:"parentId,omitempty"`
	Name           string `json:"name"`
	Role           string `json:"role"`
	LifecycleState string `json:"lifecycleState"`
	ActivityKind   string `json:"activityKind"`
	Presence       string `json:"presence"`
}

type normalizedWorld struct {
	Agents []AgentNode
}

type normalizedWorldSource interface {
	normalizedWorld() normalizedWorld
}

type WorldSnapshot struct {
	Type   string      `json:"type"`
	Agents []AgentNode `json:"agents"`
}

func newServer() http.Handler {
	staticRoot, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/api/health", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/api/tree", func(w http.ResponseWriter, r *http.Request) {
		serveTreeSelection(w, r, demoSource{})
	})
	mux.HandleFunc("/ws", serveDemoSnapshot)
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))
	return mux
}

type threadRecordSource interface {
	threadRecords() []ThreadRecord
}

type rootPickerResponse struct {
	Type  string      `json:"type"`
	Roots []AgentNode `json:"roots"`
}

type selectionErrorResponse struct {
	Error string `json:"error"`
}

func serveTreeSelection(w http.ResponseWriter, r *http.Request, source threadRecordSource) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	latest, err := parseLatestSelector(r.URL.Query().Get("latest"), r.URL.Query().Has("latest"))
	if err != nil {
		writeSelectionError(w, http.StatusBadRequest, "invalid_selector")
		return
	}
	selector := ThreadSelector{
		ThreadID: r.URL.Query().Get("thread"),
		Latest:   latest,
		CWD:      r.URL.Query().Get("cwd"),
	}
	records := source.threadRecords()
	if strings.TrimSpace(selector.ThreadID) == "" && !selector.Latest {
		roots := eligibleRoots(records, selector.CWD)
		if len(roots) == 0 {
			writeSelectionError(w, http.StatusNotFound, "no_eligible_root")
			return
		}
		picker := rootPickerResponse{Type: "root_picker", Roots: make([]AgentNode, 0, len(roots))}
		for _, root := range roots {
			picker.Roots = append(picker.Roots, safeAgentNode(root, root.ID))
		}
		writeJSON(w, http.StatusOK, picker)
		return
	}

	root, err := selectExecutionRoot(records, selector)
	if err != nil {
		status, code := selectionError(err)
		writeSelectionError(w, status, code)
		return
	}
	tree, err := reconstructExecutionTree(records, root.ID)
	if err != nil {
		status, code := selectionError(err)
		writeSelectionError(w, status, code)
		return
	}
	writeJSON(w, http.StatusOK, worldSnapshot(tree))
}

func parseLatestSelector(value string, present bool) (bool, error) {
	if !present || value == "" {
		return present, nil
	}
	return strconv.ParseBool(value)
}

func selectionError(err error) (int, string) {
	switch {
	case errors.Is(err, ErrThreadNotFound):
		return http.StatusNotFound, "thread_not_found"
	case errors.Is(err, ErrCWDMismatch):
		return http.StatusBadRequest, "cwd_mismatch"
	case errors.Is(err, ErrNoEligibleRoot):
		return http.StatusNotFound, "no_eligible_root"
	case errors.Is(err, ErrSelectorRequired):
		return http.StatusBadRequest, "selector_required"
	default:
		return http.StatusBadRequest, "invalid_selector"
	}
}

func writeSelectionError(w http.ResponseWriter, status int, code string) {
	writeJSON(w, status, selectionErrorResponse{Error: code})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func serveDemoSnapshot(w http.ResponseWriter, r *http.Request) {
	connection, err := websocket.Accept(w, r, nil)
	if err != nil {
		log.Printf("accept demo WebSocket: %v", err)
		return
	}
	defer connection.Close(websocket.StatusNormalClosure, "")

	if err := wsjson.Write(r.Context(), connection, demoSnapshot()); err != nil {
		log.Printf("write demo snapshot: %v", err)
		return
	}

	for {
		_, _, err := connection.Read(r.Context())
		if err != nil {
			return
		}
		_ = connection.Close(websocket.StatusPolicyViolation, "demo WebSocket is server-only")
		return
	}
}

func demoSnapshot() WorldSnapshot {
	return worldSnapshot(demoSource{})
}

type demoSource struct{}

func (demoSource) threadRecords() []ThreadRecord {
	return []ThreadRecord{
		{
			ID:        "root-demo",
			SessionID: "demo-session",
			CWD:       "/demo",
			Agent: AgentNode{
				ID:             "root-demo",
				Name:           "root-agent",
				Role:           "root",
				LifecycleState: "running",
				ActivityKind:   "reasoning",
				Presence:       "active",
			},
		},
		{
			ID:             "scout-demo",
			ParentThreadID: "root-demo",
			SessionID:      "demo-session",
			CWD:            "/demo",
			Agent: AgentNode{
				ID:             "scout-demo",
				Name:           "scout-1",
				Role:           "subagent",
				LifecycleState: "running",
				ActivityKind:   "research",
				Presence:       "active",
			},
		},
	}
}

func (demoSource) normalizedWorld() normalizedWorld {
	tree, err := reconstructExecutionTree(demoSource{}.threadRecords(), "root-demo")
	if err != nil {
		log.Printf("reconstruct demo tree: %v", err)
		return normalizedWorld{}
	}
	return tree.normalizedWorld()
}

func worldSnapshot(source normalizedWorldSource) WorldSnapshot {
	world := source.normalizedWorld()
	return WorldSnapshot{
		Type:   "snapshot",
		Agents: world.Agents,
	}
}

func main() {
	demo := flag.Bool("demo", false, "run the deterministic demo world")
	listen := flag.String("listen", "0.0.0.0:8040", "HTTP listen address")
	flag.Parse()

	if !*demo {
		log.Fatal("this build currently supports --demo only")
	}

	log.Printf("codex-village demo listening on http://%s", *listen)
	log.Fatal(http.ListenAndServe(*listen, newServer()))
}
