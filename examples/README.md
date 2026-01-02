# CAP Examples

Quick copy/paste messages and flows:

- `job-request.json` — minimal `BusPacket{JobRequest}` for an echo pool.
- `job-result.json` — matching `BusPacket{JobResult}`.
- `heartbeat.json` — standalone `BusPacket{Heartbeat}` advertising pool membership and load.
- `simple-echo/` — end-to-end submission + result with sequence diagram and bus messages (Go/Python/Node snippets).
- `workflow-repo-review/` — multi-step parent/child workflow with aggregation and message traces.

All timestamps are illustrative; replace IDs and pointers as needed.
