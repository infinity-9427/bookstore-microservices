package service

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/infinity-9427/bookstore-microservices/orders/internal/clients"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/config"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/models"
	"github.com/infinity-9427/bookstore-microservices/orders/internal/repository"
	"github.com/shopspring/decimal"
)

type OrdersService interface {
	CreateOrder(ctx context.Context, req *models.CreateOrderRequest, idempotencyKey string) (*models.Order, error)
	GetOrderByID(ctx context.Context, id int64) (*models.Order, error)
	ListOrders(ctx context.Context) ([]*models.Order, error)
	ListOrdersPaginated(ctx context.Context, pagination *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error)
}

type ordersService struct {
	repo        repository.OrdersRepository
	booksClient clients.BooksClient
	logger      *slog.Logger
	config      *config.Config
}

func NewOrdersService(
	repo repository.OrdersRepository,
	booksClient clients.BooksClient,
	logger *slog.Logger,
	config *config.Config,
) OrdersService {
	return &ordersService{
		repo:        repo,
		booksClient: booksClient,
		logger:      logger,
		config:      config,
	}
}

func (s *ordersService) CreateOrder(ctx context.Context, req *models.CreateOrderRequest, idempotencyKey string) (*models.Order, error) {
	requestID := ctx.Value("request_id")
	if requestID == nil {
		requestID = "unknown"
	}

	s.logger.InfoContext(ctx, "Creating order",
		slog.String("request_id", fmt.Sprintf("%v", requestID)),
		slog.Int("item_count", len(req.Items)),
		slog.String("idempotency_key", idempotencyKey))

	// Normalize the request (sum duplicate book IDs)
	if err := req.Validate(); err != nil {
		return nil, &ValidationError{Message: err.Error()}
	}

	// Check idempotency first - only if feature is enabled
	var requestHash string
	if s.config.IdempotencyEnabled && idempotencyKey != "" {
		// Create hash of the original request
		requestData, _ := json.Marshal(req.Items)
		hash := sha256.Sum256(requestData)
		requestHash = fmt.Sprintf("%x", hash)

		if existingOrder, err := s.repo.CheckIdempotencyKey(ctx, idempotencyKey, requestHash); err == nil {
			s.logger.InfoContext(ctx, "Returning existing order for idempotency key",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.String("idempotency_key", idempotencyKey),
				slog.Int64("order_id", existingOrder.ID))
			return existingOrder, nil
		} else if conflictErr, ok := err.(*repository.IdempotencyConflictError); ok {
			return nil, &IdempotencyConflictError{Key: conflictErr.Key}
		} else if _, ok := err.(*repository.OrderNotFoundError); !ok {
			// Some other error occurred
			s.logger.ErrorContext(ctx, "Failed to check idempotency",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.String("error", err.Error()))
			return nil, &InternalError{Message: "Failed to check idempotency"}
		}
		// Order not found, continue with creation
	}

	// Extract unique book IDs
	bookIDs := make([]int64, 0, len(req.Items))
	itemsMap := make(map[int64]*models.CreateOrderItemRequest)
	for _, item := range req.Items {
		if _, exists := itemsMap[item.BookID]; !exists {
			bookIDs = append(bookIDs, item.BookID)
			itemsMap[item.BookID] = &item
		}
	}

	s.logger.InfoContext(ctx, "Validating books with Books service",
		slog.String("request_id", fmt.Sprintf("%v", requestID)),
		slog.Any("book_ids", bookIDs))

	// Validate all books with the Books service (no DB transaction yet)
	books, err := s.booksClient.GetBooks(ctx, bookIDs)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to validate books",
			slog.String("request_id", fmt.Sprintf("%v", requestID)),
			slog.String("error", err.Error()))

		// Map client errors to service errors
		switch e := err.(type) {
		case *clients.BookNotFoundError:
			return nil, &BookNotFoundError{BookID: e.BookID}
		case *clients.BookInactiveError:
			return nil, &BookNotOrderableError{BookID: e.BookID}
		case *clients.CircuitBreakerError, *clients.ServiceUnavailableError:
			return nil, &ServiceUnavailableError{Message: e.Error()}
		default:
			return nil, &ServiceUnavailableError{Message: "Books service error: " + err.Error()}
		}
	}

	// Check if all requested books were found
	for _, bookID := range bookIDs {
		if _, found := books[bookID]; !found {
			return nil, &BookNotFoundError{BookID: bookID}
		}
	}

	s.logger.InfoContext(ctx, "All books validated, creating order",
		slog.String("request_id", fmt.Sprintf("%v", requestID)),
		slog.Int("books_validated", len(books)))

	// Calculate totals using exact decimal arithmetic - NO FLOATS
	orderItems := make([]models.OrderItem, 0, len(req.Items))
	orderTotal := decimal.Zero

	for _, itemReq := range req.Items {
		book := books[itemReq.BookID]

		// Parse price string directly to decimal - never use floats
		unitPrice, err := book.GetPriceDecimal()
		if err != nil {
			s.logger.ErrorContext(ctx, "Invalid price format",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.Int64("book_id", book.ID),
				slog.String("price", book.Price),
				slog.String("error", err.Error()))
			return nil, &InternalError{Message: "Invalid book price format"}
		}

		// Exact decimal multiplication: price × quantity
		quantity := decimal.NewFromInt(int64(itemReq.Quantity))
		lineTotal := unitPrice.Mul(quantity).Round(2)
		orderTotal = orderTotal.Add(lineTotal)

		orderItem := models.OrderItem{
			BookID:     itemReq.BookID,
			BookTitle:  book.Title,
			BookAuthor: book.Author,
			Quantity:   itemReq.Quantity,
			UnitPrice:  models.FormatPrice(unitPrice), // Always 2dp string
			TotalPrice: models.FormatPrice(lineTotal), // Renamed from LineTotal, always 2dp string
		}
		orderItems = append(orderItems, orderItem)
	}

	// Create the order with all calculated values
	order := &models.Order{
		Items:      orderItems,
		TotalPrice: models.FormatPrice(orderTotal), // Renamed from TotalAmount, always 2dp string
	}

	// Now begin transaction and create order
	var createErr error
	if s.config.IdempotencyEnabled {
		createErr = s.repo.CreateOrderWithIdempotency(ctx, order, idempotencyKey, requestHash)
	} else {
		createErr = s.repo.CreateOrder(ctx, order)
	}
	if createErr != nil {
		switch createErr.(type) {
		case *repository.IdempotencyConflictError:
			s.logger.WarnContext(ctx, "Idempotency key conflict",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.String("idempotency_key", idempotencyKey))
			return nil, &IdempotencyConflictError{Key: idempotencyKey}
		default:
			s.logger.ErrorContext(ctx, "Failed to create order",
				slog.String("request_id", fmt.Sprintf("%v", requestID)),
				slog.String("error", createErr.Error()))
			return nil, &InternalError{Message: "Failed to create order"}
		}
	}

	s.logger.InfoContext(ctx, "Order created successfully",
		slog.String("request_id", fmt.Sprintf("%v", requestID)),
		slog.Int64("order_id", order.ID),
		slog.String("total_price", order.TotalPrice), // Updated field name
		slog.String("idempotency_key", idempotencyKey))

	return order, nil
}

func (s *ordersService) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		switch err.(type) {
		case *repository.OrderNotFoundError:
			return nil, &OrderNotFoundError{ID: id}
		default:
			s.logger.ErrorContext(ctx, "Failed to get order", slog.Int64("order_id", id), slog.String("error", err.Error()))
			return nil, &InternalError{Message: "Failed to get order"}
		}
	}
	return order, nil
}

func (s *ordersService) ListOrders(ctx context.Context) ([]*models.Order, error) {
	orders, err := s.repo.ListOrders(ctx)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list orders", slog.String("error", err.Error()))
		return nil, &InternalError{Message: "Failed to list orders"}
	}

	if orders == nil {
		orders = make([]*models.Order, 0)
	}

	return orders, nil
}

func (s *ordersService) ListOrdersPaginated(ctx context.Context, pagination *models.PaginationRequest) (*models.PaginatedResponse[*models.Order], error) {
	orders, total, err := s.repo.ListOrdersPaginated(ctx, pagination.Limit, pagination.Offset)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list paginated orders", slog.String("error", err.Error()))
		return nil, &InternalError{Message: "Failed to list orders"}
	}

	if orders == nil {
		orders = make([]*models.Order, 0)
	}

	response := &models.PaginatedResponse[*models.Order]{
		Data:   orders,
		Total:  total,
		Limit:  pagination.Limit,
		Offset: pagination.Offset,
	}

	return response, nil
}

// Removed formatCentsAsDecimal - now using exact decimal arithmetic with shopspring/decimal

// Service error types
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

type BookNotFoundError struct {
	BookID int64
}

func (e *BookNotFoundError) Error() string {
	return fmt.Sprintf("book with ID %d not found", e.BookID)
}

type BookNotOrderableError struct {
	BookID int64
}

func (e *BookNotOrderableError) Error() string {
	return fmt.Sprintf("book with ID %d is not orderable", e.BookID)
}

type OrderNotFoundError struct {
	ID int64
}

func (e *OrderNotFoundError) Error() string {
	return fmt.Sprintf("order with ID %d not found", e.ID)
}

type ServiceUnavailableError struct {
	Message string
}

func (e *ServiceUnavailableError) Error() string {
	return e.Message
}

type IdempotencyConflictError struct {
	Key string
}

func (e *IdempotencyConflictError) Error() string {
	return fmt.Sprintf("idempotency key '%s' already used with different request body", e.Key)
}

type InternalError struct {
	Message string
}

func (e *InternalError) Error() string {
	return e.Message
}
