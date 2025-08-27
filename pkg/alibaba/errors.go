package alibaba

import (
	"errors"
	"strings"
)

// create cluster validation errors
var (
	ErrRequiredRegionID    error = errors.New("region id is required")
	ErrRequiredClusterName error = errors.New("cluster name is required")
)

func IsNotFound(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "ErrorClusterNotFound")
}
