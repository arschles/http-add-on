package main

import (
	"context"
	"encoding/json"
	"log"
	nethttp "net/http"
	"net/url"

	"github.com/kedacore/http-add-on/pkg/http"
	"github.com/kedacore/http-add-on/pkg/routing"
	echo "github.com/labstack/echo/v4"
)

// newForwardingHandler takes in the service URL for the app backend
// and forwards incoming requests to it. Note that it isn't multitenant.
// It's intended to be deployed and scaled alongside the application itself
func newQueueSizeHandler(q http.QueueCountReader) echo.HandlerFunc {
	return func(c echo.Context) error {
		cur, err := q.Current()
		if err != nil {
			log.Printf("Error getting queue size (%s)", err)
			c.Error(err)
			return err
		}
		return c.JSON(200, cur)
	}
}

func newRoutingPingHandler(
	operatorAdminURL *url.URL,
	table *routing.Table,
) echo.HandlerFunc {
	return func(c echo.Context) error {
		newTable, err := fetchRoutingTable(
			c.Request().Context(),
			operatorAdminURL,
		)
		if err != nil {
			log.Printf("error fetching new routing table (%s)", err)
			return c.String(500, "error fetching routing table")
		}
		table.Replace(newTable)
		return c.String(200, "OK")
	}
}

func fetchRoutingTable(
	ctx context.Context,
	operatorAdminURL *url.URL,
) (*routing.Table, error) {
	res, err := nethttp.Get(operatorAdminURL.String())
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	newTable := routing.NewTable()
	if err := json.NewDecoder(res.Body).Decode(newTable); err != nil {
		return nil, err
	}
	return newTable, nil
}
