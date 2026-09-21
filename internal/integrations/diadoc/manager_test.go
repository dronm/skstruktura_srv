package diadoc

import (
	"testing"
	"time"
)

func TestSelectBox(t *testing.T) {
	choices := []BoxChoice{
		{BoxID: "09ae254c5cd0408284de7ccb46d86f82@diadoc.ru"},
		{BoxID: "1f208d032a604f6491b1b7aad54cfaf3@diadoc.ru"},
	}
	boxID, ok := selectBox("09ae254c-5cd0-4082-84de-7ccb46d86f82", choices)
	if !ok || boxID != choices[0].BoxID {
		t.Fatalf("selectBox() = %q, %v", boxID, ok)
	}
	if _, ok := selectBox("", choices); ok {
		t.Fatal("selectBox() selected one of several boxes without configuration")
	}
}

func TestDotNetTicks(t *testing.T) {
	if got := dotNetTicks(time.Unix(0, 0)); got != dotNetUnixEpochTicks {
		t.Fatalf("dotNetTicks(unix epoch) = %d", got)
	}
}

func TestLimitedContent(t *testing.T) {
	result, truncated := limitedContent([]byte("12345"), 3)
	if !truncated || string(result) != "123" {
		t.Fatalf("limitedContent() = %q, %v", result, truncated)
	}
}
