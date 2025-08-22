import { Queue, QueueOptions } from 'bullmq';
import { Redis } from 'ioredis';

// Redis connection configuration
const redis = new Redis({
  host: process.env.REDIS_HOST || 'localhost',
  port: parseInt(process.env.REDIS_PORT || '6379'),
  password: process.env.REDIS_PASSWORD || undefined,
  maxRetriesPerRequest: null, // Required by BullMQ
});

// Queue configuration
const queueOptions: QueueOptions = {
  connection: redis,
  defaultJobOptions: {
    removeOnComplete: parseInt(process.env.JOB_REMOVE_ON_COMPLETE || '100'),
    removeOnFail: parseInt(process.env.JOB_REMOVE_ON_FAIL || '50'),
    attempts: 3,
    backoff: {
      type: 'exponential',
      delay: 2000,
    },
  },
};

// Create queue instance
const queueName = process.env.QUEUE_NAME || 'bookstore-jobs';
export const queue = new Queue(queueName, queueOptions);

// Job types and interfaces
export interface OrderProcessingJobData {
  orderId: string;
  bookId: string;
  quantity: number;
  customerId: string;
  totalPrice: number;
}

export interface NotificationJobData {
  recipient: string;
  type: 'email' | 'sms' | 'push';
  message: string;
  orderId?: string;
  templateData?: Record<string, any>;
}

// Queue operations
export class QueueProducer {
  
  static async addOrderProcessingJob(data: OrderProcessingJobData, delay?: number) {
    console.log(`📤 Adding order processing job for order ${data.orderId}`);
    
    return await queue.add('process-order', data, {
      delay: delay || 0,
      priority: 10, // High priority for order processing
    });
  }

  static async addNotificationJob(data: NotificationJobData, delay?: number) {
    console.log(`📤 Adding notification job for ${data.recipient}`);
    
    return await queue.add('send-notification', data, {
      delay: delay || 0,
      priority: 5, // Medium priority for notifications
    });
  }

  static async addBulkOrderProcessingJobs(orders: OrderProcessingJobData[]) {
    console.log(`📤 Adding ${orders.length} bulk order processing jobs`);
    
    const jobs = orders.map(order => ({
      name: 'process-order',
      data: order,
      opts: { priority: 10 }
    }));

    return await queue.addBulk(jobs);
  }

  static async getQueueStats() {
    const waiting = await queue.getWaiting();
    const active = await queue.getActive();
    const completed = await queue.getCompleted();
    const failed = await queue.getFailed();
    
    return {
      waiting: waiting.length,
      active: active.length,
      completed: completed.length,
      failed: failed.length,
    };
  }

  static async cleanQueue() {
    await queue.clean(5000, 100, 'completed');
    await queue.clean(10000, 50, 'failed');
    console.log('🧹 Queue cleaned');
  }
}

// Graceful shutdown
export async function closeQueue() {
  await queue.close();
  await redis.quit();
  console.log('📪 Queue connection closed');
}