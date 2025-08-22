import { Worker, Job } from 'bullmq';
import { Redis } from 'ioredis';
import { OrderProcessingJobData, NotificationJobData } from './queue.js';

// Load environment variables
const REDIS_HOST = process.env.REDIS_HOST || 'localhost';
const REDIS_PORT = parseInt(process.env.REDIS_PORT || '6379');
const REDIS_PASSWORD = process.env.REDIS_PASSWORD || undefined;
const WORKER_CONCURRENCY = parseInt(process.env.WORKER_CONCURRENCY || '5');
const QUEUE_NAME = process.env.QUEUE_NAME || 'bookstore-jobs';
const WORKER_NAME = process.env.WORKER_NAME || 'bookstore-worker';

// Redis connection configuration
const redis = new Redis({
  host: REDIS_HOST,
  port: REDIS_PORT,
  password: REDIS_PASSWORD,
  maxRetriesPerRequest: null, // Required by BullMQ
});

// Worker for processing jobs
const worker = new Worker(QUEUE_NAME, async (job: Job) => {
  const timestamp = new Date().toISOString();
  console.log(`[${timestamp}] 🔄 Processing job ${job.name} with ID ${job.id}`);
  console.log(`[${timestamp}] 📋 Job data:`, JSON.stringify(job.data, null, 2));

  try {
    switch (job.name) {
      case 'process-order':
        await processOrderJob(job);
        break;
      case 'send-notification':
        await sendNotificationJob(job);
        break;
      default:
        console.warn(`[${timestamp}] ⚠️  Unknown job type: ${job.name}`);
    }
    
    console.log(`[${timestamp}] ✅ Job ${job.id} completed successfully`);
  } catch (error) {
    console.error(`[${timestamp}] ❌ Job ${job.id} failed:`, error);
    throw error;
  }
}, {
  connection: redis,
  concurrency: WORKER_CONCURRENCY,
});

// Job processing functions
async function processOrderJob(job: Job) {
  const data: OrderProcessingJobData = job.data;
  const timestamp = new Date().toISOString();
  
  console.log(`[${timestamp}] 📦 Processing order ${data.orderId}`);
  console.log(`[${timestamp}] 📚 Book ID: ${data.bookId}, Quantity: ${data.quantity}`);
  console.log(`[${timestamp}] 👤 Customer: ${data.customerId}, Total: $${data.totalPrice}`);
  
  // Update job progress
  await job.updateProgress(10);
  
  // Simulate inventory check
  console.log(`[${timestamp}] 🔍 Checking inventory for book ${data.bookId}...`);
  await new Promise(resolve => setTimeout(resolve, 500));
  await job.updateProgress(30);
  
  // Simulate payment processing
  console.log(`[${timestamp}] 💳 Processing payment of $${data.totalPrice}...`);
  await new Promise(resolve => setTimeout(resolve, 800));
  await job.updateProgress(60);
  
  // Simulate order fulfillment
  console.log(`[${timestamp}] 📋 Preparing order for shipment...`);
  await new Promise(resolve => setTimeout(resolve, 700));
  await job.updateProgress(90);
  
  // Final step
  console.log(`[${timestamp}] 🚚 Order ${data.orderId} ready for shipping`);
  await new Promise(resolve => setTimeout(resolve, 300));
  await job.updateProgress(100);
  
  console.log(`[${timestamp}] ✅ Order ${data.orderId} processed successfully`);
  
  // Return processing result
  return {
    orderId: data.orderId,
    status: 'processed',
    processedAt: timestamp,
    totalPrice: data.totalPrice
  };
}

async function sendNotificationJob(job: Job) {
  const data: NotificationJobData = job.data;
  const timestamp = new Date().toISOString();
  
  console.log(`[${timestamp}] 📧 Sending ${data.type} notification to ${data.recipient}`);
  console.log(`[${timestamp}] 💬 Message: ${data.message}`);
  
  // Update job progress
  await job.updateProgress(20);
  
  // Simulate notification service call
  switch (data.type) {
    case 'email':
      console.log(`[${timestamp}] 📧 Connecting to email service...`);
      await new Promise(resolve => setTimeout(resolve, 400));
      break;
    case 'sms':
      console.log(`[${timestamp}] 📱 Connecting to SMS service...`);
      await new Promise(resolve => setTimeout(resolve, 300));
      break;
    case 'push':
      console.log(`[${timestamp}] 🔔 Connecting to push notification service...`);
      await new Promise(resolve => setTimeout(resolve, 200));
      break;
  }
  
  await job.updateProgress(70);
  
  // Simulate sending
  console.log(`[${timestamp}] 📤 Sending ${data.type} notification...`);
  await new Promise(resolve => setTimeout(resolve, 500));
  await job.updateProgress(100);
  
  console.log(`[${timestamp}] ✅ ${data.type} notification sent to ${data.recipient}`);
  
  return {
    recipient: data.recipient,
    type: data.type,
    sentAt: timestamp,
    orderId: data.orderId
  };
}

// Worker event handlers
worker.on('completed', (job) => {
  const timestamp = new Date().toISOString();
  console.log(`[${timestamp}] ✅ Job ${job.id} (${job.name}) completed`);
});

worker.on('failed', (job, err) => {
  const timestamp = new Date().toISOString();
  console.error(`[${timestamp}] ❌ Job ${job?.id} (${job?.name}) failed:`, err.message);
});

worker.on('error', (err) => {
  const timestamp = new Date().toISOString();
  console.error(`[${timestamp}] 🔥 Worker error:`, err);
});

worker.on('stalled', (jobId) => {
  const timestamp = new Date().toISOString();
  console.warn(`[${timestamp}] ⏰ Job ${jobId} stalled`);
});

worker.on('progress', (job, progress) => {
  const timestamp = new Date().toISOString();
  console.log(`[${timestamp}] 📊 Job ${job.id} progress: ${progress}%`);
});

// Graceful shutdown
async function shutdown() {
  const timestamp = new Date().toISOString();
  console.log(`[${timestamp}] 🛑 Shutting down worker gracefully...`);
  
  try {
    await worker.close();
    await redis.quit();
    console.log(`[${timestamp}] 👋 Worker shutdown complete`);
    process.exit(0);
  } catch (error) {
    console.error(`[${timestamp}] ❌ Error during shutdown:`, error);
    process.exit(1);
  }
}

process.on('SIGTERM', shutdown);
process.on('SIGINT', shutdown);

// Startup message
const timestamp = new Date().toISOString();
console.log(`[${timestamp}] 🚀 ${WORKER_NAME} started and waiting for jobs...`);
console.log(`[${timestamp}] 🔗 Redis: ${REDIS_HOST}:${REDIS_PORT}`);
console.log(`[${timestamp}] 📋 Queue: ${QUEUE_NAME}`);
console.log(`[${timestamp}] ⚡ Concurrency: ${WORKER_CONCURRENCY}`);