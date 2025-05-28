package params

import (
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"
)

// Optional is a helper function that can be used to fetch a requested parameter from the request.
// It does the following checks:
// 1. Checks if the parameter is present in the request, if not, it returns its zero-value
// 2. If it is present, it checks if the parameter is of the expected type and returns it
func Optional[T any](r mcp.CallToolRequest, p string) (T, error) {
	var zero T

	args := r.GetArguments()
	if args == nil {
		return zero, nil
	}

	val, ok := args[p]
	if !ok {
		return zero, nil
	}

	value, ok := val.(T)
	if !ok {
		return zero, fmt.Errorf("parameter %s is not of type %T, is %T", p, zero, val)
	}

	return value, nil
}
