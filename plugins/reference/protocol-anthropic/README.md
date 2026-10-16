# Anthropic Messages reference protocol

This is a small, independent protocol reference used by the Phase 3A
validation. It is deliberately a Python process and imports only the generated
protobuf module plus the Python gRPC runtime. It does not import the Go SDK, a
provider SDK, or any ModuAPI Go package.

The plugin accepts the subset needed to test the boundary:

- `POST /v1/messages` with `model`, non-empty text `messages`, positive
  `max_tokens`, and an optional boolean `stream`;
- non-stream JSON responses with `type: "message"` and text content;
- streaming `message_start`, `content_block_delta`, `message_delta`, and
  `message_stop` SSE events;
- input/output token usage observation;
- unknown JSON request fields are preserved because the original body is
  forwarded without re-encoding.

It intentionally does not implement the complete Anthropic API, provider
authentication, tools, images, or model compatibility. The upstream is still
the generic `upstream.http` component. The official Messages API uses the
`/v1/messages` resource and the event names exercised here; see the
[Anthropic Messages API reference](https://platform.claude.com/docs/en/api/messages/create)
and [streaming guide](https://platform.claude.com/docs/en/api/messages-streaming).

## Run through the host

The conformance command builds the Go reference components, creates a pinned
Python virtual environment, and starts this plugin as an ordinary interpreter
bundle:

```bash
make conformance
```

The test copies the bundle to a temporary directory and replaces the manifest's
`interpreter: python` with the pinned interpreter path. The checked-in manifest
therefore remains portable and package-local (`command: plugin.py`).
