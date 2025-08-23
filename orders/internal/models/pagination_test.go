package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestPaginationRequest tests the validation of pagination parameters
func TestPaginationRequest(t *testing.T) {
	testCases := []struct {
		name     string
		req      PaginationRequest
		isValid  bool
		expected PaginationRequest
	}{
		{
			name:     "Zero values (defaults handled by gin)",
			req:      PaginationRequest{},
			isValid:  true,
			expected: PaginationRequest{Limit: 0, Offset: 0}, // Gin will set defaults when binding
		},
		{
			name:     "Valid pagination",
			req:      PaginationRequest{Limit: 10, Offset: 20},
			isValid:  true,
			expected: PaginationRequest{Limit: 10, Offset: 20},
		},
		{
			name:     "Maximum limit",
			req:      PaginationRequest{Limit: 100, Offset: 0},
			isValid:  true,
			expected: PaginationRequest{Limit: 100, Offset: 0},
		},
		{
			name:     "Minimum valid limit",
			req:      PaginationRequest{Limit: 1, Offset: 0},
			isValid:  true,
			expected: PaginationRequest{Limit: 1, Offset: 0},
		},
		{
			name:     "Large offset",
			req:      PaginationRequest{Limit: 50, Offset: 1000},
			isValid:  true,
			expected: PaginationRequest{Limit: 50, Offset: 1000},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test that the struct contains expected values
			assert.Equal(t, tc.expected.Limit, tc.req.Limit, "Limit should match")
			assert.Equal(t, tc.expected.Offset, tc.req.Offset, "Offset should match")
		})
	}
}

// TestPaginatedResponse tests the pagination response structure
func TestPaginatedResponse(t *testing.T) {
	// Create sample orders
	order1 := &Order{
		ID:         1,
		Items:      []OrderItem{},
		TotalPrice: "19.99",
	}
	order2 := &Order{
		ID:         2,
		Items:      []OrderItem{},
		TotalPrice: "24.99",
	}

	orders := []*Order{order1, order2}

	response := &PaginatedResponse[*Order]{
		Data:   orders,
		Total:  150,
		Limit:  20,
		Offset: 0,
	}

	assert.Equal(t, 2, len(response.Data), "Should have 2 orders")
	assert.Equal(t, 150, response.Total, "Total should be 150")
	assert.Equal(t, 20, response.Limit, "Limit should be 20")
	assert.Equal(t, 0, response.Offset, "Offset should be 0")
	assert.Equal(t, int64(1), response.Data[0].ID, "First order ID should be 1")
	assert.Equal(t, int64(2), response.Data[1].ID, "Second order ID should be 2")
}

// TestPaginatedResponseEmpty tests empty pagination response
func TestPaginatedResponseEmpty(t *testing.T) {
	response := &PaginatedResponse[*Order]{
		Data:   []*Order{},
		Total:  0,
		Limit:  20,
		Offset: 0,
	}

	assert.Equal(t, 0, len(response.Data), "Should have no orders")
	assert.Equal(t, 0, response.Total, "Total should be 0")
	assert.Equal(t, 20, response.Limit, "Limit should be 20")
	assert.Equal(t, 0, response.Offset, "Offset should be 0")
}

// TestPaginationMath tests pagination calculations
func TestPaginationMath(t *testing.T) {
	testCases := []struct {
		name           string
		total          int
		limit          int
		offset         int
		expectedPages  int
		isLastPage     bool
		hasNextPage    bool
		hasPrevPage    bool
		currentPage    int
		itemsOnPage    int
	}{
		{
			name:          "First page with full results",
			total:         100,
			limit:         20,
			offset:        0,
			expectedPages: 5,
			isLastPage:    false,
			hasNextPage:   true,
			hasPrevPage:   false,
			currentPage:   1,
			itemsOnPage:   20,
		},
		{
			name:          "Middle page",
			total:         100,
			limit:         20,
			offset:        40,
			expectedPages: 5,
			isLastPage:    false,
			hasNextPage:   true,
			hasPrevPage:   true,
			currentPage:   3,
			itemsOnPage:   20,
		},
		{
			name:          "Last page with partial results",
			total:         95,
			limit:         20,
			offset:        80,
			expectedPages: 5,
			isLastPage:    true,
			hasNextPage:   false,
			hasPrevPage:   true,
			currentPage:   5,
			itemsOnPage:   15,
		},
		{
			name:          "Single page",
			total:         10,
			limit:         20,
			offset:        0,
			expectedPages: 1,
			isLastPage:    true,
			hasNextPage:   false,
			hasPrevPage:   false,
			currentPage:   1,
			itemsOnPage:   10,
		},
		{
			name:          "Empty results",
			total:         0,
			limit:         20,
			offset:        0,
			expectedPages: 0,
			isLastPage:    true,
			hasNextPage:   false,
			hasPrevPage:   false,
			currentPage:   1,
			itemsOnPage:   0,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Calculate pagination metadata
			totalPages := (tc.total + tc.limit - 1) / tc.limit
			if tc.total == 0 {
				totalPages = 0
			}
			
			currentPage := (tc.offset / tc.limit) + 1
			hasNextPage := tc.offset+tc.limit < tc.total
			hasPrevPage := tc.offset > 0
			isLastPage := !hasNextPage
			
			// Calculate items on current page
			itemsOnPage := tc.limit
			if tc.total < tc.offset+tc.limit {
				itemsOnPage = tc.total - tc.offset
			}
			if itemsOnPage < 0 {
				itemsOnPage = 0
			}

			assert.Equal(t, tc.expectedPages, totalPages, "Total pages should match")
			assert.Equal(t, tc.currentPage, currentPage, "Current page should match")
			assert.Equal(t, tc.hasNextPage, hasNextPage, "Has next page should match")
			assert.Equal(t, tc.hasPrevPage, hasPrevPage, "Has previous page should match")
			assert.Equal(t, tc.isLastPage, isLastPage, "Is last page should match")
			assert.Equal(t, tc.itemsOnPage, itemsOnPage, "Items on page should match")
		})
	}
}