package metrics

import (
	"crypto/tls"
	"net/http"
	"net/url"
	"strings"
	"time"

	influx "github.com/influxdata/influxdb1-client"
	"github.com/pkg/errors"
)

// InfluxClient defines values needed to push to InfluxDB
type InfluxClient struct {
	client *influx.Client
	db     string
}

// InfluxClientInput defines values needed to push to InfluxDB
type InfluxClientInput struct {
	Database string
	Config   InfluxConfig
}

// InfluxConfig is cast to an influx.Config object
type InfluxConfig struct {
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
	client, err := influx.NewClient(influx.Config(input.Config))
	if err != nil {
		return nil, errors.Wrap(err, "influx.NewClient")
	}
	return &InfluxClient{
		db:     input.Database,
		client: client,
	}, nil
}

// Push pushes a metric of name string and value of any type
func (i *InfluxClient) Push(name string, value interface{}, tags map[string]string) error {
	measurementName := strings.Replace(name, " ", "_", -1)
	batch := influx.BatchPoints{
		Database: i.db,
		Tags:     tags,
		Points: []influx.Point{
			influx.Point{
				Measurement: measurementName,
				Fields: map[string]interface{}{
					"value": value,
				},
			},
		},
	}
	_, err := i.client.Write(batch)
	return err
}
