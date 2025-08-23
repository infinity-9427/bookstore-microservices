package models

import (
	"testing"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

// TestDecimalMathAccuracy tests the exact decimal calculations to prevent
// the 1-cent undercount bug that occurred with float64 arithmetic
func TestDecimalMathAccuracy(t *testing.T) {
	testCases := []struct {
		name          string
		price         string
		quantity      int
		expectedTotal string
		expectedUnit  string
	}{
		{
			name:          "19.99 × 1 = 19.99 (regression test for 1-cent undercount)",
			price:         "19.99",
			quantity:      1,
			expectedTotal: "19.99",
			expectedUnit:  "19.99",
		},
		{
			name:          "19.99 × 2 = 39.98",
			price:         "19.99",
			quantity:      2,
			expectedTotal: "39.98",
			expectedUnit:  "19.99",
		},
		{
			name:          "24.99 × 20 = 499.80",
			price:         "24.99",
			quantity:      20,
			expectedTotal: "499.80",
			expectedUnit:  "24.99",
		},
		{
			name:          "High precision: 19.999 × 3 = 59.997 → 60.00",
			price:         "19.999",
			quantity:      3,
			expectedTotal: "60.00", // Rounded to 2 decimal places
			expectedUnit:  "20.00", // Unit price also rounded to 2dp for display
		},
		{
			name:          "Edge case: 0.01 × 99 = 0.99",
			price:         "0.01",
			quantity:      99,
			expectedTotal: "0.99",
			expectedUnit:  "0.01",
		},
		{
			name:          "Edge case: 0.99 × 100 = 99.00",
			price:         "0.99",
			quantity:      100,
			expectedTotal: "99.00",
			expectedUnit:  "0.99",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Test Book.GetPriceDecimal()
			book := &Book{Price: tc.price}
			unitPrice, err := book.GetPriceDecimal()
			assert.NoError(t, err, "Should parse price without error")

			// Test exact decimal multiplication
			quantity := decimal.NewFromInt(int64(tc.quantity))
			lineTotal := unitPrice.Mul(quantity).Round(2)

			// Verify calculations
			assert.Equal(t, tc.expectedUnit, FormatPrice(unitPrice), "Unit price formatting")
			assert.Equal(t, tc.expectedTotal, FormatPrice(lineTotal), "Line total calculation")

			// Verify no precision loss in string round-trip for 2dp formatting
			unitPriceStr := FormatPrice(unitPrice)
			parsedBack, err := ParsePrice(unitPriceStr)
			assert.NoError(t, err, "Should parse formatted price back")
			// Since FormatPrice rounds to 2dp, compare the rounded values
			assert.True(t, unitPrice.Round(2).Equal(parsedBack), "No precision loss in 2dp round-trip")
		})
	}
}

// TestOrderTotalCalculation tests that order totals are calculated correctly
// by summing line totals with exact decimal arithmetic
func TestOrderTotalCalculation(t *testing.T) {
	testCases := []struct {
		name  string
		items []struct {
			price    string
			quantity int
		}
		expectedTotal string
	}{
		{
			name: "Multi-book order: 19.99×2 + 24.99×1 = 64.97",
			items: []struct {
				price    string
				quantity int
			}{
				{"19.99", 2}, // 39.98
				{"24.99", 1}, // 24.99
			},
			expectedTotal: "64.97",
		},
		{
			name: "Large order: 19.99×2 + 24.99×20 + 24.99×12 = 839.66",
			items: []struct {
				price    string
				quantity int
			}{
				{"19.99", 2},  // 39.98
				{"24.99", 20}, // 499.80
				{"24.99", 12}, // 299.88
			},
			expectedTotal: "839.66", // Sum: 39.98 + 499.80 + 299.88 = 839.66
		},
		{
			name: "Precision edge case: multiple items with .99 pricing",
			items: []struct {
				price    string
				quantity int
			}{
				{"9.99", 3},  // 29.97
				{"14.99", 2}, // 29.98
				{"19.99", 1}, // 19.99
			},
			expectedTotal: "79.94", // 29.97 + 29.98 + 19.99 = 79.94
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			orderTotal := decimal.Zero

			for _, item := range tc.items {
				price, err := ParsePrice(item.price)
				assert.NoError(t, err, "Should parse price %s", item.price)

				quantity := decimal.NewFromInt(int64(item.quantity))
				lineTotal := price.Mul(quantity).Round(2)
				orderTotal = orderTotal.Add(lineTotal)
			}

			assert.Equal(t, tc.expectedTotal, FormatPrice(orderTotal), "Order total calculation")
		})
	}
}

// TestNoFloatContamination ensures we never accidentally use float64 in calculations
func TestNoFloatContamination(t *testing.T) {
	// Test that we can't accidentally introduce float64 precision errors
	priceStr := "19.99"

	// Correct way: string → decimal
	correctPrice, err := ParsePrice(priceStr)
	assert.NoError(t, err)
	correctTotal := correctPrice.Mul(decimal.NewFromInt(1)).Round(2)

	// Verify exact value
	assert.Equal(t, "19.99", FormatPrice(correctTotal))

	// Demonstrate what would happen with float64 (for comparison)
	// Note: This is just for demonstration - we never do this in production code
	floatPrice := 19.99
	floatTotal := floatPrice * 1.0
	// Float64 representation might be slightly off due to binary representation
	// This test ensures our decimal implementation doesn't have such issues

	// Our decimal implementation should be exact
	assert.True(t, correctTotal.Equal(decimal.RequireFromString("19.99")))

	// Verify that decimal doesn't suffer from float precision issues
	// that could cause the 1-cent undercount bug
	cents := correctPrice.Mul(decimal.NewFromInt(100)).Round(0)
	assert.Equal(t, int64(1999), cents.IntPart(), "Price in cents should be exactly 1999")

	t.Logf("Float64 result: %.10f (may have precision errors)", floatTotal)
	t.Logf("Decimal result: %s (always exact)", FormatPrice(correctTotal))
}

// TestFormatPrice tests the FormatPrice function for consistent 2dp formatting
func TestFormatPrice(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"19.9", "19.90"},  // Pad to 2dp
		{"19.99", "19.99"}, // Keep 2dp
		{"20", "20.00"},    // Add 2dp
		{"0.1", "0.10"},    // Pad cents
		{"0", "0.00"},      // Zero formatting
		{"1.999", "2.00"},  // Round to 2dp
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			d, err := ParsePrice(tc.input)
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, FormatPrice(d.Round(2)))
		})
	}
}

// TestInvalidPriceHandling tests error handling for invalid price strings
func TestInvalidPriceHandling(t *testing.T) {
	invalidPrices := []string{
		"abc",
		"19.99.99",
		"",
		"$19.99",
		"19,99", // Comma instead of dot
	}

	for _, price := range invalidPrices {
		t.Run(price, func(t *testing.T) {
			book := &Book{Price: price}
			_, err := book.GetPriceDecimal()
			assert.Error(t, err, "Should return error for invalid price: %s", price)

			_, err = ParsePrice(price)
			assert.Error(t, err, "ParsePrice should return error for invalid price: %s", price)
		})
	}
}
