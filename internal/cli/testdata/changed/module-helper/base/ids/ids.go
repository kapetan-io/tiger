package ids

import "strings"

func Clean(id string) string {
	return strings.TrimSpace(id)
}
