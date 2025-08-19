package alibaba

import "strings"

func IsNotFound(err error) bool {
	return strings.Contains(err.Error(), "ErrorClusterNotFound")
}
