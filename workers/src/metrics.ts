import express from 'express';
import { Registry, collectDefaultMetrics, Counter, Histogram } from 'prom-client';

export const registry = new Registry();
collectDefaultMetrics({ register: registry });

export const jobsCompleted = new Counter({
  name: 'worker_jobs_completed_total',
  help: 'Jobs completed',
  labelNames: ['queue'],
  registers: [registry],
});

export const jobsFailed = new Counter({
  name: 'worker_jobs_failed_total',
  help: 'Jobs failed by class',
  labelNames: ['queue', 'class'],
  registers: [registry],
});

export const jobDuration = new Histogram({
  name: 'worker_job_duration_seconds',
  help: 'Job processing time',
  labelNames: ['queue'],
  buckets: [0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10],
  registers: [registry],
});

export function startMetricsServer(port = Number(process.env.WORKER_METRICS_PORT) || 9100) {
  const app = express();
  app.get('/metrics', async (_req: any, res: any) => {
    res.set('Content-Type', registry.contentType);
    res.end(await registry.metrics());
  });
  app.listen(port, () => console.log(JSON.stringify({ level: 'INFO', msg: 'worker metrics listening', port })));
}
