package order

import (
	"errors"
	"testing"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name       string
		customerID string
		items      []Item
		wantTotal  int64
		wantErr    bool
	}{
		{
			name:       "valid single item",
			customerID: "c1",
			items:      []Item{{SKU: "ABC", Qty: 2, PriceCents: 1999}},
			wantTotal:  3998,
		},
		{
			name:       "valid multiple items",
			customerID: "c1",
			items: []Item{
				{SKU: "ABC", Qty: 2, PriceCents: 1999},
				{SKU: "XYZ", Qty: 3, PriceCents: 500},
			},
			wantTotal: 5498,
		},
		{
			name:       "empty customer",
			customerID: "",
			items:      []Item{{SKU: "ABC", Qty: 1, PriceCents: 100}},
			wantErr:    true,
		},
		{
			name:       "empty items",
			customerID: "c1",
			items:      nil,
			wantErr:    true,
		},
		{
			name:       "zero qty",
			customerID: "c1",
			items:      []Item{{SKU: "ABC", Qty: 0, PriceCents: 100}},
			wantErr:    true,
		},
		{
			name:       "negative price",
			customerID: "c1",
			items:      []Item{{SKU: "ABC", Qty: 1, PriceCents: -1}},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			o, err := New(tt.customerID, tt.items)

			if tt.wantErr {
				var validationErr ValidationError
				if !errors.As(err, &validationErr) {
					t.Fatalf("error = %v, want ValidationError", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if o.TotalCents != tt.wantTotal {
				t.Errorf("total_cents = %d, want %d", o.TotalCents, tt.wantTotal)
			}
			if o.Status != StatusPlaced {
				t.Errorf("status = %q, want %q", o.Status, StatusPlaced)
			}
			if o.ID == "" {
				t.Error("id is empty")
			}
			if o.CustomerID != tt.customerID {
				t.Errorf("customer_id = %q, want %q", o.CustomerID, tt.customerID)
			}
		})
	}
}
