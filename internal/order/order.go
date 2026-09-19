package order

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const StatusPlaced = "PLACED"

type Item struct {
	SKU        string `json:"sku"`
	Qty        int64  `json:"qty"`
	PriceCents int64  `json:"price_cents"`
}

type Order struct {
	ID         string    `json:"id"`
	CustomerID string    `json:"customer_id"`
	Items      []Item    `json:"items"`
	TotalCents int64     `json:"total_cents"`
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
}

type ValidationError struct {
	Message string
}

func (e ValidationError) Error() string {
	return e.Message
}

func New(customerID string, items []Item) (Order, error) {
	if customerID == "" {
		return Order{}, ValidationError{Message: "customer_id is required"}
	}
	if len(items) == 0 {
		return Order{}, ValidationError{Message: "at least one item is required"}
	}

	var totalCents int64
	for _, item := range items {
		if item.Qty <= 0 {
			return Order{}, ValidationError{Message: "item qty must be greater than zero"}
		}
		if item.PriceCents < 0 {
			return Order{}, ValidationError{Message: "item price_cents must not be negative"}
		}
		totalCents += item.Qty * item.PriceCents
	}

	id, err := uuid.NewV7()
	if err != nil {
		return Order{}, fmt.Errorf("generate id: %w", err)
	}

	return Order{
		ID:         id.String(),
		CustomerID: customerID,
		Items:      items,
		TotalCents: totalCents,
		Status:     StatusPlaced,
	}, nil
}
