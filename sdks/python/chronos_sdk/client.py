"""Chronos Python SDK client."""

from __future__ import annotations

import json
from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Optional
from urllib.error import HTTPError
from urllib.request import Request, urlopen


class ChronosError(Exception):
    """Base exception for Chronos SDK errors."""

    def __init__(self, message: str, status_code: int = 0, code: str = ""):
        super().__init__(message)
        self.status_code = status_code
        self.code = code


@dataclass
class Job:
    """Represents a Chronos job."""

    id: str = ""
    name: str = ""
    schedule: str = ""
    timezone: str = ""
    description: str = ""
    enabled: bool = True
    webhook_url: str = ""
    webhook_method: str = "GET"
    webhook_body: str = ""
    webhook_headers: dict[str, str] = field(default_factory=dict)
    timeout: str = ""
    namespace: str = ""
    tags: dict[str, str] = field(default_factory=dict)
    max_retries: int = 3
    created_at: str = ""
    updated_at: str = ""


@dataclass
class Execution:
    """Represents a job execution."""

    id: str = ""
    job_id: str = ""
    job_name: str = ""
    status: str = ""
    attempts: int = 0
    status_code: int = 0
    duration: float = 0.0
    error: str = ""
    started_at: str = ""
    completed_at: str = ""
    trace_id: str = ""


class ChronosClient:
    """Client for the Chronos API.

    Usage:
        client = ChronosClient("http://localhost:8080", api_key="your-key")
        job = client.create_job(
            name="daily-report",
            schedule="0 9 * * *",
            webhook_url="https://api.example.com/report",
        )
        client.trigger(job.id)
    """

    def __init__(
        self,
        base_url: str = "http://localhost:8080",
        api_key: str = "",
        namespace: str = "default",
        timeout: int = 30,
    ):
        self.base_url = base_url.rstrip("/")
        self.api_key = api_key
        self.namespace = namespace
        self.timeout = timeout

    def create_job(
        self,
        name: str,
        schedule: str,
        webhook_url: str,
        method: str = "POST",
        body: str = "",
        headers: Optional[dict[str, str]] = None,
        timeout: str = "5m",
        max_retries: int = 3,
        tags: Optional[dict[str, str]] = None,
        enabled: bool = True,
    ) -> Job:
        """Create a new scheduled job."""
        payload: dict[str, Any] = {
            "name": name,
            "schedule": schedule,
            "webhook": {"url": webhook_url, "method": method},
            "enabled": enabled,
            "namespace": self.namespace,
        }
        if body:
            payload["webhook"]["body"] = body
        if headers:
            payload["webhook"]["headers"] = headers
        if timeout:
            payload["timeout"] = timeout
        if tags:
            payload["tags"] = tags
        if max_retries > 0:
            payload["retry_policy"] = {
                "max_attempts": max_retries,
                "initial_interval": "1s",
                "max_interval": "30s",
                "multiplier": 2.0,
            }

        data = self._request("POST", "/api/v1/jobs", payload)
        return self._parse_job(data.get("data", data))

    def get_job(self, job_id: str) -> Job:
        """Get a job by ID."""
        data = self._request("GET", f"/api/v1/jobs/{job_id}")
        return self._parse_job(data.get("data", data))

    def list_jobs(self) -> list[Job]:
        """List all jobs."""
        data = self._request("GET", "/api/v1/jobs")
        inner = data.get("data", data)
        jobs_data = inner.get("jobs", []) if isinstance(inner, dict) else []
        return [self._parse_job(j) for j in jobs_data]

    def delete_job(self, job_id: str) -> None:
        """Delete a job."""
        self._request("DELETE", f"/api/v1/jobs/{job_id}")

    def trigger(self, job_id: str) -> Execution:
        """Trigger a job execution."""
        data = self._request("POST", f"/api/v1/jobs/{job_id}/trigger")
        return self._parse_execution(data.get("data", data))

    def enable(self, job_id: str) -> None:
        """Enable a job."""
        self._request("POST", f"/api/v1/jobs/{job_id}/enable")

    def disable(self, job_id: str) -> None:
        """Disable a job."""
        self._request("POST", f"/api/v1/jobs/{job_id}/disable")

    def get_executions(self, job_id: str, limit: int = 20) -> list[Execution]:
        """Get recent executions for a job."""
        data = self._request("GET", f"/api/v1/jobs/{job_id}/executions?limit={limit}")
        inner = data.get("data", data)
        execs = inner.get("executions", []) if isinstance(inner, dict) else []
        return [self._parse_execution(e) for e in execs]

    def _request(self, method: str, path: str, body: Any = None) -> dict:
        url = self.base_url + path
        data = json.dumps(body).encode() if body else None
        req = Request(url, data=data, method=method)
        req.add_header("Content-Type", "application/json")
        if self.api_key:
            req.add_header("Authorization", f"Bearer {self.api_key}")

        try:
            with urlopen(req, timeout=self.timeout) as resp:
                return json.loads(resp.read().decode())
        except HTTPError as e:
            body_text = e.read().decode() if e.fp else ""
            try:
                err_data = json.loads(body_text)
                err_info = err_data.get("error", {})
                raise ChronosError(
                    err_info.get("message", body_text),
                    status_code=e.code,
                    code=err_info.get("code", ""),
                ) from e
            except (json.JSONDecodeError, AttributeError):
                raise ChronosError(body_text, status_code=e.code) from e

    @staticmethod
    def _parse_job(data: Any) -> Job:
        if not isinstance(data, dict):
            return Job()
        webhook = data.get("webhook", {}) or {}
        return Job(
            id=data.get("id", ""),
            name=data.get("name", ""),
            schedule=data.get("schedule", ""),
            timezone=data.get("timezone", ""),
            description=data.get("description", ""),
            enabled=data.get("enabled", True),
            webhook_url=webhook.get("url", ""),
            webhook_method=webhook.get("method", "GET"),
            namespace=data.get("namespace", ""),
            tags=data.get("tags") or {},
            created_at=data.get("created_at", ""),
            updated_at=data.get("updated_at", ""),
        )

    @staticmethod
    def _parse_execution(data: Any) -> Execution:
        if not isinstance(data, dict):
            return Execution()
        return Execution(
            id=data.get("id", ""),
            job_id=data.get("job_id", ""),
            job_name=data.get("job_name", ""),
            status=data.get("status", ""),
            attempts=data.get("attempts", 0),
            status_code=data.get("status_code", 0),
            duration=data.get("duration", 0),
            error=data.get("error", ""),
            started_at=data.get("started_at", ""),
            completed_at=data.get("completed_at", ""),
            trace_id=data.get("trace_id", ""),
        )
