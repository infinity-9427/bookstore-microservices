import { Queue } from 'bullmq';
import { Redis } from 'ioredis';

// Environment
const ORDERS_API_URL = process.env.ORDERS_API_URL || 'http://orders:8082';
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = parseInt(process.env.REDIS_PORT || '6379');
const QUEUE_NAME = process.env.ORDERS_QUEUE_NAME || 'orders-create';

// Redis connection
const redis = new Redis({ host: REDIS_HOST, port: REDIS_PORT, maxRetriesPerRequest: null });
const queue = new Queue(QUEUE_NAME, { connection: redis, defaultJobOptions: { attempts: 0, removeOnComplete: 1000, removeOnFail: 1000 } });

interface CreateOrderJob {
  idempotencyKey?: string;
  payload: any;
  expect?: string; // for test labeling
}

// Helper to generate idempotency keys
function idem(key: string) { return `it-${key}`; }

async function main() {
  const n = parseInt(process.env.ENQUEUE_TOTAL || '20');
  console.log(`Enqueuing ${n} mixed order creation jobs to queue '${QUEUE_NAME}' ...`);

  // Example book IDs for valid / invalid mixes (adjust as needed after seeding with 1..3 active books)
  const jobs: CreateOrderJob[] = [];

  for (let i = 0; i < n; i++) {
    const mod = i % 8;
    switch (mod) {
      case 0: // valid single
        jobs.push({ payload: { items: [{ book_id: 1, quantity: 1 }] }, idempotencyKey: idem(`valid-${i}`), expect: '201' });
        break;
      case 1: // valid multi + duplicate merge
        jobs.push({ payload: { items: [{ book_id: 2, quantity: 1 }, { book_id: 2, quantity: 2 }] }, idempotencyKey: idem(`merge-${i}`), expect: '201-merged' });
        break;
      case 2: // not found
        jobs.push({ payload: { items: [{ book_id: 999999, quantity: 1 }] }, idempotencyKey: idem(`nf-${i}`), expect: '404' });
        break;
      case 3: // inactive (simulate book_id 3 will be soft-deleted in setup)
        jobs.push({ payload: { items: [{ book_id: 3, quantity: 1 }] }, idempotencyKey: idem(`inactive-${i}`), expect: '409' });
        break;
      case 4: // multiple books including one not found -> expect first error semantics => 404
        jobs.push({ payload: { items: [{ book_id: 1, quantity: 1 }, { book_id: 42424242, quantity: 2 }] }, idempotencyKey: idem(`mixed-${i}`), expect: '404' });
        break;
      case 5: // idempotent retry same body (enqueue twice with same key)
        const key = idem(`same-${i}`);
        jobs.push({ payload: { items: [{ book_id: 1, quantity: 2 }] }, idempotencyKey: key, expect: '201-first' });
        jobs.push({ payload: { items: [{ book_id: 1, quantity: 2 }] }, idempotencyKey: key, expect: '200-idempotent' });
        break;
      case 6: // idempotent different body -> conflict 409
        const diffKey = idem(`diff-${i}`);
        jobs.push({ payload: { items: [{ book_id: 1, quantity: 1 }] }, idempotencyKey: diffKey, expect: '201-first' });
        jobs.push({ payload: { items: [{ book_id: 1, quantity: 2 }] }, idempotencyKey: diffKey, expect: '409-conflict' });
        break;
      default: // induced 5xx via bogus books service (set env in processor to point to invalid) – here we mark expected 503
        jobs.push({ payload: { items: [{ book_id: 1, quantity: 1 }] }, idempotencyKey: idem(`503-${i}`), expect: '503' });
    }
  }

  // Enqueue
  for (const j of jobs) {
    await queue.add('create-order', j, { jobId: j.idempotencyKey ? `${j.idempotencyKey}-${Math.random()}` : undefined });
  }

  console.log(`Enqueued ${jobs.length} jobs.`);
  await queue.close();
  await redis.quit();
}

main().catch(err => { console.error(err); process.exit(1); });
