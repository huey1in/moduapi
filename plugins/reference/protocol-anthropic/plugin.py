"""Messages reference: wire-only Python, no provider client or Go SDK.

Text Messages JSON/SSE pass-through, usage observation, no OpenAI translation.
Unknown JSON fields remain byte-for-byte intact. See README for strict subset.
"""
import hmac
import json
import os
import threading
from concurrent import futures

import grpc
from google.protobuf.any_pb2 import Any
from google.rpc import status_pb2
from grpc_status import rpc_status
from moduapi.plugin.v1 import plugin_pb2 as pb
from moduapi.plugin.v1 import plugin_pb2_grpc as rpc

LIMIT = 4 << 20
CHUNK = 32768


def strict_json(raw):
    def pairs(items):
        result = {}
        for key, value in items:
            if key in result:
                raise ValueError("duplicate key")
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=pairs,
                      parse_constant=lambda _: (_ for _ in ()).throw(ValueError()))


def failure(code, kind="upstream_protocol", status=502, usage=None):
    return pb.Frame(failure=pb.Failure(kind=kind, code=code,
                    message="Messages reference rejected input", status=status, usage=usage))


def reject(ctx, code, status=400):
    detail = Any()
    detail.Pack(failure(code, "local_rejection", status).failure)
    ctx.abort_with_status(rpc_status.to_status(status_pb2.Status(
        code=grpc.StatusCode.INVALID_ARGUMENT.value[0], message="invalid Messages request",
        details=[detail])))


def integer(v):
    return type(v) is int and 0 <= v <= (1 << 63) - 1


class Messages(rpc.LifecycleServicer, rpc.ProtocolServicer):
    def __init__(self):
        self.lock = threading.Lock()
        self.ready = False
        self.draining = False
        self.finished = threading.Event()

    def admitted(self, ctx):
        if not self.ready or self.draining:
            ctx.abort(grpc.StatusCode.UNAVAILABLE, "not accepting work")

    def Handshake(self, req, ctx):
        if req.api_version != 1:
            ctx.abort(grpc.StatusCode.FAILED_PRECONDITION, "unsupported API")
        return pb.Description(id="protocol.anthropic-reference", version="0.1.0",
                              api_version=1, provides=["protocol"],
                              capabilities=[c for c in ["streaming", "usage"] if c in req.capabilities],
                              endpoints=[pb.Endpoint(method="POST", path="/v1/messages",
                                                     protocol="anthropic.messages")])

    def Configure(self, req, ctx):
        with self.lock:
            if self.ready or self.draining:
                ctx.abort(grpc.StatusCode.FAILED_PRECONDITION, "already configured")
            try:
                if strict_json(req.json) != {}:
                    raise ValueError()
            except (ValueError, UnicodeError):
                ctx.abort(grpc.StatusCode.INVALID_ARGUMENT, "empty object required")
            self.ready = True
        return pb.Empty()

    def Health(self, req, ctx):
        return pb.HealthStatus(ready=self.ready and not self.draining)

    def Shutdown(self, req, ctx):
        with self.lock:
            self.draining = True
            self.ready = False
        def drain():
            self.server.stop(2).wait()
            self.finished.set()
        threading.Thread(target=drain, daemon=True).start()
        return pb.Empty()

    def Decode(self, req, ctx):
        self.admitted(ctx)
        headers = {k.lower(): list(v.values) for k, v in req.headers.items()}
        if headers.get("content-type", [""])[0].split(";")[0].strip().lower() != "application/json":
            reject(ctx, "unsupported_media_type", 415)
        if req.method != "POST" or req.path != "/v1/messages":
            reject(ctx, "invalid_endpoint")
        try:
            obj = strict_json(req.body)
            model = obj["model"]
            messages = obj["messages"]
            if (not isinstance(model, str) or not model.strip() or len(model.encode()) > 256
                    or any(c in model for c in "\r\n\x00") or not integer(obj["max_tokens"])
                    or obj["max_tokens"] == 0 or not isinstance(messages, list) or not messages
                    or type(obj.get("stream", False)) is not bool):
                raise ValueError()
            for message in messages:
                if message["role"] not in ("user", "assistant"):
                    raise ValueError()
                content = message["content"]
                if isinstance(content, str):
                    continue
                if not isinstance(content, list) or not content:
                    raise ValueError()
                for block in content:
                    if block["type"] != "text" or not isinstance(block["text"], str):
                        raise ValueError()
        except (ValueError, TypeError, KeyError, UnicodeError):
            reject(ctx, "invalid_messages")
        stream = obj.get("stream", False)
        return pb.Request(id=req.id, protocol="anthropic.messages", model=model,
                          stream=stream, method="POST", path="/messages", body=req.body,
                          headers={"Content-Type": pb.Values(values=["application/json"]),
                                   "Accept": pb.Values(values=["text/event-stream" if stream else "application/json"]),
                                   "Anthropic-Version": pb.Values(values=["2023-06-01"])})

    def Prepare(self, req, ctx):
        self.admitted(ctx)
        try:
            if req.options_json and strict_json(req.options_json) != {}:
                raise ValueError()
        except (ValueError, UnicodeError):
            reject(ctx, "invalid_protocol_config", 500)
        # CopyFrom preserves unknown protobuf fields; body is never re-encoded.
        result = pb.Request()
        result.CopyFrom(req.request)
        return result

    def Transform(self, inputs, ctx):
        self.admitted(ctx)
        usage = pb.Usage()
        started = terminal = message_started = message_stopped = False
        pending = bytearray()
        body = bytearray()
        success = stream = False
        try:
            first = next(inputs)
            if first.WhichOneof("value") != "request":
                raise ValueError()
            req = first.request
            for item in inputs:
                if not ctx.is_active():
                    return
                if item.WhichOneof("value") != "frame" or terminal:
                    raise ValueError()
                f = item.frame
                kind = f.WhichOneof("value")
                if kind == "start":
                    if started or not 200 <= f.start.status <= 599:
                        raise ValueError()
                    started = True
                    success = f.start.status < 300
                    stream = success and req.stream
                    if stream:
                        headers = {k.lower(): v for k, v in f.start.headers.items()}
                        if headers.get("content-type", pb.Values()).values[0].split(";")[0] != "text/event-stream":
                            raise ValueError()
                elif kind == "data":
                    if not started or len(f.data) > CHUNK:
                        raise ValueError()
                    if stream:
                        pending.extend(f.data)
                        pending = bytearray(pending.replace(b"\r\n", b"\n"))
                        if len(pending) > LIMIT:
                            raise ValueError()
                        while b"\n\n" in pending:
                            event, _, tail = pending.partition(b"\n\n")
                            pending = bytearray(tail)
                            lines = event.replace(b"\r", b"").split(b"\n")
                            data = b"\n".join(line[5:].lstrip() for line in lines if line.startswith(b"data:"))
                            if not data:
                                continue
                            obj = strict_json(data)
                            name = obj["type"]
                            if message_stopped:
                                raise ValueError()
                            if name == "message_start":
                                if message_started:
                                    raise ValueError()
                                message_started = True
                                self.observe(usage, obj["message"].get("usage", {}))
                            elif name == "message_delta":
                                if not message_started:
                                    raise ValueError()
                                self.observe(usage, obj.get("usage", {}))
                            elif name == "message_stop":
                                if not message_started:
                                    raise ValueError()
                                message_stopped = True
                            elif name == "error":
                                usage.partial = True
                                yield failure("messages_stream_error", usage=usage)
                                return
                            # Future event types are forwarded, not interpreted as OpenAI.
                    elif success:
                        body.extend(f.data)
                        if len(body) > LIMIT:
                            raise ValueError()
                elif kind in ("end", "failure"):
                    if kind == "end" and not started:
                        raise ValueError()
                    terminal = True
                    # Require clean source EOF before emitting a terminal.
                    if next(inputs, None) is not None:
                        raise ValueError()
                    if kind == "failure":
                        yield f
                        return
                    if success:
                        if stream:
                            if not message_stopped or pending.strip():
                                raise ValueError()
                        else:
                            obj = strict_json(body)
                            if obj["type"] != "message" or not isinstance(obj["content"], list):
                                raise ValueError()
                            self.observe(usage, obj.get("usage", {}))
                    yield pb.Frame(end=pb.ResponseEnd(usage=usage))
                    return
                else:
                    raise ValueError()
                yield f
            raise ValueError()
        except (ValueError, KeyError, TypeError, IndexError, StopIteration, UnicodeError):
            usage.partial = True
            yield failure("invalid_messages_response", usage=usage)
        except grpc.RpcError:
            if ctx.is_active():
                yield failure("messages_stream_transport", usage=usage)

    @staticmethod
    def observe(usage, obj):
        for key in ("input_tokens", "output_tokens", "cache_read_input_tokens"):
            if key in obj:
                if not integer(obj[key]):
                    raise ValueError()
                setattr(usage, "cached_tokens" if key == "cache_read_input_tokens" else key, obj[key])


class Authentication(grpc.ServerInterceptor):
    def intercept_service(self, continuation, details):
        token = os.environ.get("MODUAPI_PLUGIN_TOKEN", "")
        got = [v for k, v in details.invocation_metadata if k == "authorization"]
        if len(got) != 1 or not hmac.compare_digest(got[0], "Bearer " + token):
            def denied(req, ctx):
                ctx.abort(grpc.StatusCode.UNAUTHENTICATED, "host token required")
            return grpc.unary_unary_rpc_method_handler(denied)
        return continuation(details)


def main():
    if len(os.environ.get("MODUAPI_PLUGIN_TOKEN", "")) < 32:
        raise SystemExit("launch through a host")
    ref = Messages()
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=16),
                         interceptors=[Authentication()],
                         options=[("grpc.max_receive_message_length", 10 << 20),
                                  ("grpc.max_send_message_length", 10 << 20)])
    ref.server = server
    rpc.add_LifecycleServicer_to_server(ref, server)
    rpc.add_ProtocolServicer_to_server(ref, server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    print(json.dumps({"api_version": 1, "address": f"127.0.0.1:{port}"}), flush=True)
    def eof():
        while os.read(0, 4096):
            pass
        ref.draining = True
        server.stop(0).wait()
        ref.finished.set()
    threading.Thread(target=eof, daemon=True).start()
    ref.finished.wait()


if __name__ == "__main__":
    main()
