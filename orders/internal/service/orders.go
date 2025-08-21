package service

import (
	"context"
	"fmt"
	"log/slog"
	"strconv"

	"github.com/yourname/bookstore-microservices/orders/internal/clients"
	"github.com/yourname/bookstore-microservices/orders/internal/models"
	"github.com/yourname/bookstore-microservices/orders/internal/repository"
)

type OrdersService interface {
	CreateOrder(ctx context.Context, req *models.CreateOrderRequest) (*models.Order, error)
	GetOrderByID(ctx context.Context, id int64) (*models.Order, error)
	ListOrders(ctx context.Context, query *models.ListOrdersQuery) ([]*models.Order, error)
}

type ordersService struct {
	repo        repository.OrdersRepository
	booksClient clients.BooksClient
	logger      *slog.Logger
}

func NewOrdersService(
	repo repository.OrdersRepository,
	booksClient clients.BooksClient,
	logger *slog.Logger,
) OrdersService {
	return &ordersService{
		repo:        repo,
		booksClient: booksClient,
		logger:      logger,
	}
}

func (s *ordersService) CreateOrder(ctx context.Context, req *models.CreateOrderRequest) (*models.Order, error) {
	requestID := getRequestID(ctx)
	
	s.logger.InfoContext(ctx, "Creating order", 
		slog.String("request_id", requestID),
		slog.Int64("book_id", req.BookID),
		slog.Int("quantity", req.Quantity),
	)
	
	if err := req.Validate(); err != nil {
		s.logger.WarnContext(ctx, "Invalid order request", 
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, &ValidationError{Message: err.Error()}
	}
	
	book, err := s.booksClient.GetBook(ctx, req.BookID)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to get book from books service", 
			slog.String("request_id", requestID),
			slog.Int64("book_id", req.BookID),
			slog.String("error", err.Error()),
		)
		
		switch err.(type) {
		case *clients.BookNotFoundError:
			return nil, &BookNotFoundError{BookID: req.BookID}
		case *clients.BookNotActiveError:
			return nil, &BookNotActiveError{BookID: req.BookID}
		default:
			return nil, &ServiceUnavailableError{Message: "Books service is currently unavailable"}
		}
	}
	
	unitPrice, err := strconv.ParseFloat(book.Price, 64)
	if err != nil {
		s.logger.ErrorContext(ctx, "Invalid book price format", 
			slog.String("request_id", requestID),
			slog.Int64("book_id", req.BookID),
			slog.String("price", book.Price),
			slog.String("error", err.Error()),
		)
		return nil, &InternalError{Message: "Invalid book price format"}
	}

	order := &models.Order{
		BookID:     book.ID,
		BookTitle:  book.Title,
		BookAuthor: book.Author,
		Quantity:   req.Quantity,
		UnitPrice:  unitPrice,
	}
	
	if err := s.repo.CreateOrder(ctx, order); err != nil {
		s.logger.ErrorContext(ctx, "Failed to create order in database", 
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, &InternalError{Message: "Failed to create order"}
	}
	
	s.logger.InfoContext(ctx, "Order created successfully", 
		slog.String("request_id", requestID),
		slog.Int64("order_id", order.ID),
		slog.Float64("total_price", order.TotalPrice),
	)
	
	return order, nil
}

func (s *ordersService) GetOrderByID(ctx context.Context, id int64) (*models.Order, error) {
	requestID := getRequestID(ctx)
	
	s.logger.InfoContext(ctx, "Getting order by ID", 
		slog.String("request_id", requestID),
		slog.Int64("order_id", id),
	)
	
	order, err := s.repo.GetOrderByID(ctx, id)
	if err != nil {
		switch err.(type) {
		case *repository.OrderNotFoundError:
			s.logger.WarnContext(ctx, "Order not found", 
				slog.String("request_id", requestID),
				slog.Int64("order_id", id),
			)
			return nil, &OrderNotFoundError{ID: id}
		default:
			s.logger.ErrorContext(ctx, "Failed to get order from database", 
				slog.String("request_id", requestID),
				slog.Int64("order_id", id),
				slog.String("error", err.Error()),
			)
			return nil, &InternalError{Message: "Failed to get order"}
		}
	}
	
	return order, nil
}

func (s *ordersService) ListOrders(ctx context.Context, query *models.ListOrdersQuery) ([]*models.Order, error) {
	requestID := getRequestID(ctx)
	
	query.SetDefaults()
	
	s.logger.InfoContext(ctx, "Listing orders", 
		slog.String("request_id", requestID),
		slog.Int("limit", query.Limit),
		slog.Int("offset", query.Offset),
	)
	
	orders, err := s.repo.ListOrders(ctx, query.Limit, query.Offset)
	if err != nil {
		s.logger.ErrorContext(ctx, "Failed to list orders from database", 
			slog.String("request_id", requestID),
			slog.String("error", err.Error()),
		)
		return nil, &InternalError{Message: "Failed to list orders"}
	}
	
	s.logger.InfoContext(ctx, "Orders listed successfully", 
		slog.String("request_id", requestID),
		slog.Int("count", len(orders)),
	)
	
	return orders, nil
}

func getRequestID(ctx context.Context) string {
	if requestID := ctx.Value("request_id"); requestID != nil {
		return requestID.(string)
	}
	return "unknown"
}

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

type BookNotActiveError struct {
	BookID int64
}

func (e *BookNotActiveError) Error() string {
	return fmt.Sprintf("book with ID %d is not active", e.BookID)
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

type InternalError struct {
	Message string
}

func (e *InternalError) Error() string {
	return e.Message
}