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

func demoSnapshot() map[string]any {
	return map[string]any{
		"type": "snapshot",
		"agents": []map[string]string{
			{
				"id":             "root-demo",
				"name":           "root-agent",
				"role":           "root",
				"lifecycleState": "running",
				"activityKind":   "reasoning",
				"presence":       "active",
			},
			{
				"id":             "scout-demo",
				"parentId":       "root-demo",
				"name":           "scout-1",
				"role":           "subagent",
				"lifecycleState": "running",
				"activityKind":   "research",
				"presence":       "active",
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
