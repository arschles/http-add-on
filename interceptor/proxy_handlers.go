package main

import (
	"net/http"
	"net/http/httputil"
	"net/url"

	echo "github.com/labstack/echo/v4"
)

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself
func newForwardingHandler(fwdSvcURL *url.URL) echo.HandlerFunc {
	return func(c echo.Context) error {
		r := c.Request()
		c.Logger().Debug("Incoming request for %s (%v)", *fwdSvcURL, *r)

		proxy := httputil.NewSingleHostReverseProxy(fwdSvcURL)
		proxy.Director = func(req *http.Request) {
			req.URL = fwdSvcURL
			req.Host = fwdSvcURL.Host
			req.URL.Path = r.URL.Path
			req.URL.RawQuery = r.URL.RawQuery
		}

		w := c.Response()
		proxy.ServeHTTP(w, r)
		return nil
	}
}
