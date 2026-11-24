package ioinloop

import "go/types"

var noIOCalls = map[string]bool{
	"net.SplitHostPort": true,
	"net.JoinHostPort":  true,
	"net.ParseIP":       true,
	"net.ParseCIDR":     true,
	"net.ParseMAC":      true,

	"(*net.IPNet).Contains": true,
	"(*net.IPNet).String":   true,

	"os.Getenv": true,

	"os.IsExist":      true,
	"os.IsNotExist":   true,
	"os.IsPermission": true,
	"os.IsTimeout":    true,

	"(*os.File).Name": true,

	"net/http.StatusText":         true,
	"net/http.CanonicalHeaderKey": true,
	"net/http.ParseHTTPVersion":   true,

	"(*net/http.Request).Context": true,
}

func noIO(fn *types.Func) bool {
	return noIOCalls[fn.FullName()]
}
