import { QueueProducer, OrderProcessingJobData, NotificationJobData } from './queue.js';

async function testProducer() {
  console.log('🧪 Starting queue producer test...');

  try {
    // Test Order Processing Jobs
    console.log('\n📦 Creating order processing jobs...');
    
    const orderJob1: OrderProcessingJobData = {
      orderId: 'ORD-001',
      bookId: 'BOOK-123',
      quantity: 2,
      customerId: 'CUST-456',
      totalPrice: 29.98
    };

    const orderJob2: OrderProcessingJobData = {
      orderId: 'ORD-002',
      bookId: 'BOOK-789',
      quantity: 1,
      customerId: 'CUST-789',
      totalPrice: 19.99
    };

    // Add individual order jobs
    await QueueProducer.addOrderProcessingJob(orderJob1);
    await QueueProducer.addOrderProcessingJob(orderJob2, 5000); // Delayed by 5 seconds

    // Test bulk order jobs
    const bulkOrders: OrderProcessingJobData[] = [
      {
        orderId: 'ORD-003',
        bookId: 'BOOK-111',
        quantity: 3,
        customerId: 'CUST-111',
        totalPrice: 44.97
      },
      {
        orderId: 'ORD-004',
        bookId: 'BOOK-222',
        quantity: 1,
        customerId: 'CUST-222',
        totalPrice: 15.99
      }
    ];

    await QueueProducer.addBulkOrderProcessingJobs(bulkOrders);

    // Test Notification Jobs
    console.log('\n📧 Creating notification jobs...');
    
    const emailNotification: NotificationJobData = {
      recipient: 'customer@example.com',
      type: 'email',
      message: 'Your order ORD-001 has been processed successfully!',
      orderId: 'ORD-001',
      templateData: {
        customerName: 'John Doe',
        orderTotal: '$29.98'
      }
    };

    const smsNotification: NotificationJobData = {
      recipient: '+1234567890',
      type: 'sms',
      message: 'Order ORD-002 is being prepared for shipping.',
      orderId: 'ORD-002'
    };

    const pushNotification: NotificationJobData = {
      recipient: 'user-device-token-123',
      type: 'push',
      message: 'Your order is ready for pickup!',
      orderId: 'ORD-003'
    };

    await QueueProducer.addNotificationJob(emailNotification);
    await QueueProducer.addNotificationJob(smsNotification, 3000); // Delayed by 3 seconds
    await QueueProducer.addNotificationJob(pushNotification);

    // Check queue stats
    console.log('\n📊 Queue statistics:');
    const stats = await QueueProducer.getQueueStats();
    console.log(`Waiting: ${stats.waiting}`);
    console.log(`Active: ${stats.active}`);
    console.log(`Completed: ${stats.completed}`);
    console.log(`Failed: ${stats.failed}`);

    console.log('\n✅ Test producer completed successfully!');
    console.log('👀 Check the worker logs to see job processing...');

  } catch (error) {
    console.error('❌ Error in test producer:', error);
  }
}

// Run the test
testProducer();