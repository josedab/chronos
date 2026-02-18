/**
 * Chronos JavaScript/TypeScript SDK
 *
 * @example
 * ```typescript
 * import { ChronosClient } from '@chronos/sdk';
 *
 * const client = new ChronosClient('http://localhost:8080', { apiKey: 'your-key' });
 * const job = await client.createJob({
 *   name: 'daily-report',
 *   schedule: '0 9 * * *',
 *   webhookUrl: 'https://api.example.com/report',
 *   method: 'POST',
 * });
 * await client.trigger(job.id);
 * ```
 */

export interface Job {
  id: string;
  name: string;
  schedule: string;
  timezone?: string;
  description?: string;
  enabled: boolean;
  webhook?: { url: string; method: string; headers?: Record<string, string>; body?: string };
  namespace?: string;
  tags?: Record<string, string>;
  created_at?: string;
  updated_at?: string;
}

export interface Execution {
  id: string;
  job_id: string;
  job_name?: string;
  status: 'pending' | 'running' | 'success' | 'failed' | 'skipped';
  attempts: number;
  status_code?: number;
  duration?: number;
  error?: string;
  started_at: string;
  completed_at?: string;
  trace_id?: string;
}

export interface CreateJobOptions {
  name: string;
  schedule: string;
  webhookUrl: string;
  method?: string;
  body?: string;
  headers?: Record<string, string>;
  timeout?: string;
  maxRetries?: number;
  tags?: Record<string, string>;
  enabled?: boolean;
  namespace?: string;
}

export interface ChronosClientOptions {
  apiKey?: string;
  namespace?: string;
  timeout?: number;
}

export class ChronosError extends Error {
  statusCode: number;
  code: string;

  constructor(message: string, statusCode: number = 0, code: string = '') {
    super(message);
    this.name = 'ChronosError';
    this.statusCode = statusCode;
    this.code = code;
  }
}

export class ChronosClient {
  private baseUrl: string;
  private apiKey: string;
  private namespace: string;
  private timeout: number;

  constructor(baseUrl: string = 'http://localhost:8080', options: ChronosClientOptions = {}) {
    this.baseUrl = baseUrl.replace(/\/$/, '');
    this.apiKey = options.apiKey || '';
    this.namespace = options.namespace || 'default';
    this.timeout = options.timeout || 30000;
  }

  async createJob(opts: CreateJobOptions): Promise<Job> {
    const payload: Record<string, unknown> = {
      name: opts.name,
      schedule: opts.schedule,
      webhook: {
        url: opts.webhookUrl,
        method: opts.method || 'POST',
        ...(opts.body && { body: opts.body }),
        ...(opts.headers && { headers: opts.headers }),
      },
      enabled: opts.enabled ?? true,
      namespace: opts.namespace || this.namespace,
      ...(opts.timeout && { timeout: opts.timeout }),
      ...(opts.tags && { tags: opts.tags }),
    };

    if (opts.maxRetries && opts.maxRetries > 0) {
      payload.retry_policy = {
        max_attempts: opts.maxRetries,
        initial_interval: '1s',
        max_interval: '30s',
        multiplier: 2.0,
      };
    }

    const data = await this.request('POST', '/api/v1/jobs', payload);
    return data.data as Job;
  }

  async getJob(jobId: string): Promise<Job> {
    const data = await this.request('GET', `/api/v1/jobs/${jobId}`);
    return data.data as Job;
  }

  async listJobs(): Promise<Job[]> {
    const data = await this.request('GET', '/api/v1/jobs');
    const inner = data.data as { jobs: Job[] };
    return inner.jobs || [];
  }

  async deleteJob(jobId: string): Promise<void> {
    await this.request('DELETE', `/api/v1/jobs/${jobId}`);
  }

  async trigger(jobId: string): Promise<Execution> {
    const data = await this.request('POST', `/api/v1/jobs/${jobId}/trigger`);
    return data.data as Execution;
  }

  async enable(jobId: string): Promise<void> {
    await this.request('POST', `/api/v1/jobs/${jobId}/enable`);
  }

  async disable(jobId: string): Promise<void> {
    await this.request('POST', `/api/v1/jobs/${jobId}/disable`);
  }

  async getExecutions(jobId: string, limit: number = 20): Promise<Execution[]> {
    const data = await this.request('GET', `/api/v1/jobs/${jobId}/executions?limit=${limit}`);
    const inner = data.data as { executions: Execution[] };
    return inner.executions || [];
  }

  private async request(method: string, path: string, body?: unknown): Promise<{ success: boolean; data?: unknown; error?: { code: string; message: string } }> {
    const url = this.baseUrl + path;
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (this.apiKey) {
      headers['Authorization'] = `Bearer ${this.apiKey}`;
    }

    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), this.timeout);

    try {
      const response = await fetch(url, {
        method,
        headers,
        body: body ? JSON.stringify(body) : undefined,
        signal: controller.signal,
      });

      const data = await response.json();

      if (!response.ok) {
        const err = data.error || {};
        throw new ChronosError(
          err.message || `HTTP ${response.status}`,
          response.status,
          err.code || '',
        );
      }

      return data;
    } finally {
      clearTimeout(timeoutId);
    }
  }
}

export default ChronosClient;
