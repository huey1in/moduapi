"""Wire-only reference fixture, intentionally not a Python ModuAPI SDK."""
import argparse
import hmac
import json
import os
import sys
import threading
import time
from concurrent import futures

import grpc
from google.protobuf.any_pb2 import Any
from google.rpc import status_pb2
from grpc_status import rpc_status
from moduapi.plugin.v1 import plugin_pb2 as pb
from moduapi.plugin.v1 import plugin_pb2_grpc as rpc

MAX_CHUNK = 32768
MAX_MESSAGE = 10 << 20


def varint(n):
    out = bytearray()
    while n > 127:
        out.append((n & 127) | 128)
        n >>= 7
    out.append(n)
    return bytes(out)


def unknown(number):
    return varint(number << 3) + varint(7)


class Reference(rpc.LifecycleServicer, rpc.ProtocolServicer,
                rpc.UpstreamServicer, rpc.SchedulerServicer):
    def __init__(self, args):
        self.args = args
        self.ready = False
        self.draining = False
        self.lock = threading.Lock()
        self.finished = threading.Event()

    def event(self, name):
        if self.args.events:
            with self.lock, open(self.args.events, "a", encoding="utf-8", newline="\n") as f:
                f.write(name + "\n")

    def work(self, ctx):
        if not self.ready or self.draining:
            ctx.abort(grpc.StatusCode.UNAVAILABLE, "not accepting work")

    def Handshake(self, req, ctx):
        if req.api_version != 1:
            ctx.abort(grpc.StatusCode.FAILED_PRECONDITION, "unsupported API")
        caps = [c for c in ["streaming", "affinity"] if c in req.capabilities]
        return pb.Description(id="test.python" if self.args.mode != "identity" else "test.other",
                              version="0.1.0" if self.args.mode != "version" else "0.2.0",
                              api_version=2 if self.args.mode == "api" else 1,
                              provides=["protocol", "upstream", "scheduler"],
                              capabilities=[] if self.args.mode == "capabilities" else caps,
                              endpoints=[pb.Endpoint(method="POST", path="/test", protocol="test.echo")])

    def Configure(self, req, ctx):
        try:
            obj = json.loads(req.json)
            if not isinstance(obj, dict) or obj.get("reject"):
                raise ValueError()
        except (ValueError, UnicodeDecodeError):
            ctx.abort(grpc.StatusCode.INVALID_ARGUMENT, "configuration rejected")
        if self.ready or self.draining:
            ctx.abort(grpc.StatusCode.FAILED_PRECONDITION, "already configured")
        self.ready = True
        return pb.Empty()

    def Health(self, req, ctx):
        return pb.HealthStatus(ready=self.ready and not self.draining)

    def Shutdown(self, req, ctx):
        self.draining = True
        self.ready = False
        self.event("shutdown")
        def drain():
            self.server.stop(2).wait()
            self.finished.set()
        threading.Thread(target=drain, daemon=True).start()
        return pb.Empty()

    def Decode(self, req, ctx):
        self.work(ctx)
        self.error(req.body.decode("utf-8", errors="replace"), ctx)
        return pb.Request(id=req.id, protocol="test.echo", model=req.body.decode() or "normal",
                          stream=True, method="POST", path="/test", body=req.body)

    def Prepare(self, req, ctx):
        self.work(ctx)
        out = pb.Request()
        out.CopyFrom(req.request)
        if unknown(50000) in req.request.SerializeToString():
            out.metadata["unknown_seen"] = "yes"
        out.ParseFromString(out.SerializeToString() + unknown(50001))
        return out

    def error(self, mode, ctx):
        if mode.startswith("grpc:"):
            ctx.abort(getattr(grpc.StatusCode, mode[5:]), "fixture status")
        if mode.startswith("detail:"):
            detail = Any()
            detail.Pack(pb.Failure(kind=mode[7:], code="fixture", message="fixture failure", status=502))
            ctx.abort_with_status(rpc_status.to_status(status_pb2.Status(code=3, message="fixture", details=[detail])))

    def frames(self, mode):
        start = pb.Frame(start=pb.ResponseStart(status=200))
        data = pb.Frame(data=b"data: {}\n\n")
        end = pb.Frame(end=pb.ResponseEnd())
        failure = pb.Frame(failure=pb.Failure(kind="upstream_transport", code="fixture", message="fixture failure", status=502))
        return {
            "normal": [start, data, data, end],
            "empty_data": [start, pb.Frame(data=b""), data, end],
            "failed": [start, data, failure],
            "data_first": [data, end], "duplicate_start": [start, start, end],
            "end_first": [end], "multiple_end": [start, end, end],
            "data_after_end": [start, end, data], "failure_after_end": [start, end, failure],
            "no_terminal": [start, data],
            "oversized": [start, pb.Frame(data=b"x" * (MAX_CHUNK + 1)), end],
        }.get(mode, [start, data, end])

    def Execute(self, req, ctx):
        self.work(ctx)
        self.event("execute_started")
        cancelled = threading.Event()
        ctx.add_callback(cancelled.set)
        try:
            mode = req.request.model
            self.error(mode, ctx)
            if mode.startswith("failure:"):
                yield pb.Frame(failure=pb.Failure(kind=mode[8:], code="fixture", message="fixture failure", status=502))
                return
            if mode in ("hang", "drain"):
                yield pb.Frame(start=pb.ResponseStart(status=200))
                yield pb.Frame(data=b"data: {}\n\n")
                if mode == "drain":
                    cancelled.wait(0.3)
                    if ctx.is_active():
                        yield pb.Frame(end=pb.ResponseEnd())
                else:
                    cancelled.wait()
                    self.event("execute_cancelled")
                return
            yield from self.frames(mode)
        finally:
            self.event("execute_exited")

    def Transform(self, requests, ctx):
        self.work(ctx)
        self.event("transform_started")
        try:
            first = next(requests, None)
            if first is None or first.WhichOneof("value") != "request":
                ctx.abort(grpc.StatusCode.INVALID_ARGUMENT, "request required")
            started = terminal = False
            pending = None
            for req in requests:
                if terminal or req.WhichOneof("value") != "frame":
                    raise ValueError()
                f = req.frame
                kind = f.WhichOneof("value")
                if kind == "start":
                    if started or not 200 <= f.start.status <= 599:
                        raise ValueError()
                    started = True
                elif kind == "data":
                    if not started or len(f.data) > MAX_CHUNK:
                        raise ValueError()
                elif kind == "end":
                    if not started:
                        raise ValueError()
                    terminal = True
                elif kind == "failure":
                    terminal = True
                else:
                    raise ValueError()
                if terminal:
                    pending = f
                else:
                    yield f
            if not terminal:
                raise ValueError()
            yield pending
        except ValueError:
            yield pb.Frame(failure=pb.Failure(kind="upstream_protocol", code="invalid_frame", message="invalid sequence", status=502))
        finally:
            self.event("transform_exited")

    def Select(self, req, ctx):
        self.work(ctx)
        for c in req.candidates:
            if c.id not in req.excluded_ids:
                return pb.Selection(candidate_id=c.id, reason="reference", lease=b"reference-lease")
        ctx.abort(grpc.StatusCode.RESOURCE_EXHAUSTED, "no candidate")

    def Report(self, req, ctx):
        self.work(ctx)
        self.event("report")
        return pb.Empty()


class Authentication(grpc.ServerInterceptor):
    def intercept_service(self, continuation, details):
        token = os.environ.get("MODUAPI_PLUGIN_TOKEN", "")
        values = [v for k, v in details.invocation_metadata if k == "authorization"]
        if len(values) != 1 or not hmac.compare_digest(values[0], "Bearer " + token):
            def reject(req, ctx):
                ctx.abort(grpc.StatusCode.UNAUTHENTICATED, "invalid host token")
            return grpc.unary_unary_rpc_method_handler(reject)
        return continuation(details)


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", default="normal")
    parser.add_argument("--events")
    args = parser.parse_args()
    if len(os.environ.get("MODUAPI_PLUGIN_TOKEN", "")) < 32:
        raise SystemExit("host required")
    ref = Reference(args)
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=16),
                         interceptors=[Authentication()],
                         options=[("grpc.max_receive_message_length", MAX_MESSAGE),
                                  ("grpc.max_send_message_length", MAX_MESSAGE)])
    ref.server = server
    for register in [rpc.add_LifecycleServicer_to_server, rpc.add_ProtocolServicer_to_server,
                     rpc.add_UpstreamServicer_to_server, rpc.add_SchedulerServicer_to_server]:
        register(ref, server)
    port = server.add_insecure_port("127.0.0.1:0")
    server.start()
    print(json.dumps({"api_version": 1, "address": f"127.0.0.1:{port}"}), flush=True)
    def parent_eof():
        while os.read(0, 4096):
            pass
        ref.draining = True
        server.stop(0).wait()
        ref.event("stdin_eof")
        ref.finished.set()
    threading.Thread(target=parent_eof, daemon=True).start()
    ref.finished.wait()
    ref.event("process_exited")


if __name__ == "__main__":
    main()
