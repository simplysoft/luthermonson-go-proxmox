package proxmox

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func rawStatus(code int, reason string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, _ := w.(http.Hijacker).Hijack()
		defer conn.Close()
		fmt.Fprintf(buf, "HTTP/1.1 %d %s\r\nContent-Length: 0\r\nConnection: close\r\n\r\n", code, reason)
		_ = buf.Flush()
	}
}

func TestServerErrorsAreStatusErrors(t *testing.T) {
	for _, tc := range []struct {
		code   int
		reason string
	}{
		{500, "storage 'x' does not exist"},
		{501, "Method 'GET /cluster/sdn/vnets' not implemented"},
		{595, "Errors during connection establishment, proxy handler: No route to host"},
	} {
		srv := httptest.NewServer(rawStatus(tc.code, tc.reason))
		c := NewClient(srv.URL + "/api2/json")
		var v any
		err := c.Get(context.Background(), "/anything", &v)
		srv.Close()

		var se *StatusError
		if !errors.As(err, &se) || se.StatusCode != tc.code {
			t.Fatalf("%d: err = %#v, want *StatusError", tc.code, err)
		}
		if want := fmt.Sprintf("%d %s", tc.code, tc.reason); err.Error() != want {
			t.Fatalf("%d: Error() = %q, want %q", tc.code, err.Error(), want)
		}
	}
}
