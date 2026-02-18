"""Tests for the Chronos Python SDK."""

import json
import unittest
from http.server import HTTPServer, BaseHTTPRequestHandler
from threading import Thread

from chronos_sdk.client import ChronosClient, ChronosError, Job, Execution


class MockHandler(BaseHTTPRequestHandler):
    jobs = {}

    def do_POST(self):
        if self.path == "/api/v1/jobs":
            length = int(self.headers.get("Content-Length", 0))
            body = json.loads(self.rfile.read(length)) if length else {}
            job_id = f"job-{body.get('name', 'unknown')}"
            self.jobs[job_id] = {**body, "id": job_id}
            self._respond(201, {"success": True, "data": {"id": job_id, **body}})
        elif "/trigger" in self.path:
            self._respond(200, {"success": True, "data": {"id": "exec-1", "status": "running", "job_id": "j1"}})
        elif "/enable" in self.path or "/disable" in self.path:
            self._respond(200, {"success": True})
        else:
            self._respond(404, {"success": False, "error": {"message": "not found"}})

    def do_GET(self):
        if self.path == "/api/v1/jobs":
            self._respond(200, {"success": True, "data": {"jobs": list(self.jobs.values())}})
        elif self.path.startswith("/api/v1/jobs/") and "/executions" not in self.path:
            job_id = self.path.split("/")[-1]
            if job_id in self.jobs:
                self._respond(200, {"success": True, "data": self.jobs[job_id]})
            else:
                self._respond(404, {"success": False, "error": {"code": "not_found", "message": "not found"}})
        elif "/executions" in self.path:
            self._respond(200, {"success": True, "data": {"executions": []}})
        else:
            self._respond(404, {"success": False})

    def do_DELETE(self):
        self._respond(200, {"success": True})

    def _respond(self, status, body):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(body).encode())

    def log_message(self, format, *args):
        pass


class TestChronosClient(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        MockHandler.jobs = {}
        cls.server = HTTPServer(("127.0.0.1", 0), MockHandler)
        cls.thread = Thread(target=cls.server.serve_forever, daemon=True)
        cls.thread.start()
        port = cls.server.server_address[1]
        cls.client = ChronosClient(f"http://127.0.0.1:{port}", api_key="test-key")

    @classmethod
    def tearDownClass(cls):
        cls.server.shutdown()

    def test_create_job(self):
        job = self.client.create_job(name="test-job", schedule="0 9 * * *", webhook_url="https://x.com/hook")
        self.assertEqual(job.name, "test-job")
        self.assertNotEqual(job.id, "")

    def test_list_jobs(self):
        self.client.create_job(name="list-test", schedule="* * * * *", webhook_url="http://x")
        jobs = self.client.list_jobs()
        self.assertGreater(len(jobs), 0)

    def test_get_job_not_found(self):
        with self.assertRaises(ChronosError) as ctx:
            self.client.get_job("nonexistent-id")
        self.assertEqual(ctx.exception.status_code, 404)

    def test_trigger(self):
        exec_result = self.client.trigger("job-test")
        self.assertEqual(exec_result.status, "running")

    def test_delete_job(self):
        self.client.delete_job("job-test")

    def test_get_executions(self):
        execs = self.client.get_executions("job-test")
        self.assertIsInstance(execs, list)

    def test_job_dataclass(self):
        job = Job(id="1", name="test", schedule="* * * * *")
        self.assertEqual(job.id, "1")
        self.assertTrue(job.enabled)

    def test_execution_dataclass(self):
        exe = Execution(id="e1", job_id="j1", status="success")
        self.assertEqual(exe.status, "success")

    def test_client_trailing_slash(self):
        c = ChronosClient("http://localhost:8080/")
        self.assertEqual(c.base_url, "http://localhost:8080")


if __name__ == "__main__":
    unittest.main()
