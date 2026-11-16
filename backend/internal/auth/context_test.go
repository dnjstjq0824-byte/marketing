package auth

import (
	"context"
	"testing"
)

// W20-2: the operator identity round-trips through the request context.
func TestW20OperatorContext(t *testing.T) {
	ctx := WithOperator(context.Background(), 7, "admin")
	id, name, ok := OperatorFrom(ctx)
	if !ok || id != 7 || name != "admin" {
		t.Fatalf("got %d %q %v", id, name, ok)
	}
	if OperatorName(ctx) != "admin" {
		t.Fatal("OperatorName must return the username")
	}
	if _, _, ok := OperatorFrom(context.Background()); ok {
		t.Fatal("empty context must not carry an operator")
	}
	if OperatorName(context.Background()) != "" {
		t.Fatal("empty context actor must be blank (store records system)")
	}
}
