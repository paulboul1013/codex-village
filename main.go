package main

import (
	"embed"
	"encoding/json"
	"flag"
	"io/fs"
	"log"
	"net/http"

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
	mux.HandleFunc("/ws", serveDemoSnapshot)
	mux.Handle("/", http.FileServer(http.FS(staticRoot)))
	return mux
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
	return WorldSnapshot{
		Type:   "snapshot",
		Agents: demoWorld().Agents,
	}
}

func demoWorld() normalizedWorld {
	return normalizedWorld{
		Agents: []AgentNode{
			{
				ID:             "root-demo",
				Name:           "root-agent",
				Role:           "root",
				LifecycleState: "running",
				ActivityKind:   "reasoning",
				Presence:       "active",
			},
			{
				ID:             "scout-demo",
				ParentID:       "root-demo",
				Name:           "scout-1",
				Role:           "subagent",
				LifecycleState: "running",
				ActivityKind:   "research",
				Presence:       "active",
			},
		},
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
