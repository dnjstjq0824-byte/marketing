package auth

import "context"

// Operator identity travels on the request context so audited handlers
// (W20-2) can record who acted without taking new dependencies.

type operatorKey struct{}

// WithOperator returns a context carrying the operator id and username.
func WithOperator(ctx context.Context, userID int64, username string) context.Context {
	return context.WithValue(ctx, operatorKey{}, [2]any{userID, username})
}

// OperatorFrom reports the operator carried by the context.
func OperatorFrom(ctx context.Context) (userID int64, username string, ok bool) {
	v, ok := ctx.Value(operatorKey{}).([2]any)
	if !ok {
		return 0, "", false
	}
	id, _ := v[0].(int64)
	name, _ := v[1].(string)
	return id, name, true
}

// OperatorName is the audit actor for a context: the operator username,
// or "" when the call is not interactive (the store records "system").
func OperatorName(ctx context.Context) string {
	_, name, _ := OperatorFrom(ctx)
	return name
}
