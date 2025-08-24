import { describe, it, expect } from 'vitest';

// Simple smoke test – assumes worker metrics server is already running (start processor first).
describe('worker /metrics', () => {
  it('exposes Prometheus metrics', async () => {
    const res = await fetch('http://localhost:9100/metrics');
    expect(res.status).toBe(200);
    const body = await res.text();
    expect(body).toContain('worker_jobs_completed_total');
    expect(body).toContain('worker_jobs_failed_total');
    expect(body).toContain('worker_job_duration_seconds');
  });
});