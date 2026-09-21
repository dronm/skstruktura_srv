package diadoc

import "testing"

func TestConvertedGrossPriceKeepsSixDecimals(t *testing.T) {
	t.Parallel()

	quantity := multiplyDecimals("3", "1", 4)
	price := divideDecimals("100.00", quantity, 6)

	if quantity != "3.0000" {
		t.Fatalf("quantity = %s, want 3.0000", quantity)
	}
	if price != "33.333333" {
		t.Fatalf("price = %s, want 33.333333", price)
	}
}
