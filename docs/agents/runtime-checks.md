# Local runtime checks

## Start the server

The repository requires Go 1.27. The installed toolchain is under `$HOME/.local/go1.27.0`; expose it to the current Bash session before starting the server:

```bash
export PATH="$HOME/.local/go1.27.0/bin:$PATH"
go version
go run . --latest
```

Keep that terminal open. Startup is complete only when it prints:

```text
codex-village observer listening on http://0.0.0.0:8040
```

Use `go run . --demo` when deterministic demo agents are required instead of local Codex sessions.

## Test URLs

From a Windows browser or Windows PowerShell:

```text
http://localhost:8040
http://localhost:8040/api/health
http://localhost:8040/api/tree?latest=true
```

From the same interactive WSL environment, try `http://localhost:8040/api/health` first. A Codex command sandbox may have a separate loopback namespace even though it runs under WSL. In that case, resolve the WSL `eth0` address dynamically and use it instead:

```bash
WSL_CODEX_VILLAGE_IP="$(hostname -I | awk '{print $1}')"
curl -sS --max-time 3 "http://$WSL_CODEX_VILLAGE_IP:8040/api/health"
curl -sS --max-time 3 "http://$WSL_CODEX_VILLAGE_IP:8040/api/tree?latest=true" |
  jq '{type,error,agentCount:(.agents|length)}'
```

The WSL address can change after restart; derive it for every check rather than recording a literal IP.

Expected health response:

```json
{"status":"ok"}
```

An Observer tree check succeeds when `type` is `snapshot`, `error` is `null`, and `agentCount` is at least one for an eligible local session.

## Diagnose conflicting results

During the 2026-09-08 check, a request from the Codex sandbox to `127.0.0.1:8040` failed and its process listing could not see the server, while Windows `localhost:8040` and the WSL `eth0` address both returned healthy responses. The cause was process/network namespace isolation, not a stopped server.

Use this order before concluding that the server exited:

1. Confirm the launch terminal still shows the running command and has not returned to a shell prompt.
2. Check Windows `http://localhost:8040/api/health`.
3. Check the dynamically resolved WSL address as shown above.
4. Confirm `/api/tree?latest=true` returns a snapshot.

Only diagnose a server exit when the launch command has returned or both host-visible health checks fail. Capture the final launch-terminal output when it exits unexpectedly.
