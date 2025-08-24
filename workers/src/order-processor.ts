import { Worker, Job, QueueEvents } from 'bullmq';
import { jobsCompleted, jobsFailed, jobDuration, startMetricsServer } from './metrics.js';
import { Redis } from 'ioredis';
// Node 18+ global fetch is available; types from @types/node suffice. Using RequestInit type from lib.dom.

const ORDERS_API_URL = process.env.ORDERS_API_URL || 'http://orders:8082';
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = parseInt(process.env.REDIS_PORT || '6379');
const QUEUE_NAME = process.env.ORDERS_QUEUE_NAME || 'orders-create';
const CONCURRENCY = parseInt(process.env.WORKER_CONCURRENCY || '10');
const HTTP_TIMEOUT_MS = parseInt(process.env.ORDERS_HTTP_TIMEOUT_MS || '3000');
const MAX_RETRY_5XX = parseInt(process.env.ORDERS_HTTP_RETRIES || '3');

const redis = new Redis({ host: REDIS_HOST, port: REDIS_PORT, maxRetriesPerRequest: null });

interface CreateOrderJob {
  idempotencyKey?: string;
  payload: any;
  expect?: string;
}

async function httpJson(path: string, body: any, idempotencyKey?: string): Promise<{ status: number; json: any; }> {
  const ctrl = new AbortController();
  const t = setTimeout(() => ctrl.abort(), HTTP_TIMEOUT_MS);
  try {
    const init: RequestInit = {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Request-ID': `wrk-${Date.now()}-${Math.random()}`,
        ...(idempotencyKey ? { 'Idempotency-Key': idempotencyKey } : {})
      },
      body: JSON.stringify(body),
      signal: ctrl.signal
    };
    const res = await fetch(`${ORDERS_API_URL}${path}`, init);
    let data: any = null;
    try { data = await res.json(); } catch { /* ignore parse errors */ }
    return { status: res.status, json: data };
  } finally { clearTimeout(t); }
}

async function processCreate(job: Job<CreateOrderJob>) {
  const { idempotencyKey, payload } = job.data;
  let attempt = 0;
  while (true) {
    attempt++;
    const { status, json } = await httpJson('/v1/orders', payload, idempotencyKey);
    if (status >= 500 && attempt <= MAX_RETRY_5XX) {
      await job.updateProgress(Math.min(95, attempt * (90 / MAX_RETRY_5XX)));
      continue; // retry on 5xx only
    }
    if (status >= 500) throw new Error(`Upstream 5xx after ${attempt} attempts: ${status}`);
    // 4xx not retried
    return { status, body: json };
  }
}

startMetricsServer();

const worker = new Worker<CreateOrderJob>(QUEUE_NAME, async (job) => {
  switch (job.name) {
    case 'create-order':
      return await processCreate(job);
    default:
      throw new Error(`Unknown job name ${job.name}`);
  }
}, { connection: redis, concurrency: CONCURRENCY });

// metrics classification helpers
function classify(err: unknown): '4xx' | '5xx' | 'timeout' | 'other' {
  const m = String(err instanceof Error ? err.message : err);
  if (m.toLowerCase().includes('timeout')) return 'timeout';
  const code = m.match(/\b(4\d{2}|5\d{2})\b/)?.[0];
  if (code?.startsWith('4')) return '4xx';
  if (code?.startsWith('5')) return '5xx';
  return 'other';
}

const events = new QueueEvents(QUEUE_NAME, { connection: redis });
worker.on('active', (job) => {
  // @ts-ignore augment job for timer handle
  job.__endTimer = jobDuration.labels(job.queueName).startTimer();
});
worker.on('completed', (job) => {
  // @ts-ignore
  job.__endTimer?.();
  jobsCompleted.labels(job.queueName).inc();
  console.log(`[processor] job ${job.id} completed`);
});
worker.on('failed', (job, err) => {
  // @ts-ignore
  job?.__endTimer?.();
  jobsFailed.labels(job?.queueName || 'unknown', classify(err)).inc();
  console.error(`[processor] job ${job?.id} failed: ${err?.message}`);
});

async function shutdown() { await worker.close(); await events.close(); await redis.quit(); }
process.on('SIGINT', () => shutdown().then(()=>process.exit(0)));
process.on('SIGTERM', () => shutdown().then(()=>process.exit(0)));

console.log(`Order processor started: queue=${QUEUE_NAME} concurrency=${CONCURRENCY} orders_api=${ORDERS_API_URL}`);
