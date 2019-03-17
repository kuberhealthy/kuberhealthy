package metrics

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"time"

	influx "github.com/influxdata/influxdb1-client"
	"github.com/pkg/errors"
)

// InfluxClient defines values needed to push to InfluxDB
type InfluxClient struct {
	client *influx.Client
}

// InfluxClientInput defines values needed to push to InfluxDB
type InfluxClientInput struct {
	URL              url.URL
	UnixSocket       string
	Username         string
	Password         string
	UserAgent        string
	Timeout          time.Duration
	Precision        string
	WriteConsistency string
	UnsafeSsl        bool
	Proxy            func(req *http.Request) (*url.URL, error)
	TLS              *tls.Config
}

// NewInfluxClient creates an InfluxClient that can be used to push metrics
func NewInfluxClient(input InfluxClientInput) (*InfluxClient, error) {
	client, err := influx.NewClient(influx.Config(input))
	if err != nil {
		return nil, errors.Wrap(err, "influx.NewClient")
	}
	return &InfluxClient{
		client: client,
	}, nil
}

// Push pushes a metric of name string and value of any type
func (i *InfluxClient) Push(name string, value interface{}, tags map[string]string) error {
	batch := influx.BatchPoints{
		Tags: tags,
		Points: []influx.Point{
			influx.Point{
				Fields: map[string]interface{}{
					name: value,
				},
			},
		},
	}
	_, err := i.client.Write(batch)
	return err
}
