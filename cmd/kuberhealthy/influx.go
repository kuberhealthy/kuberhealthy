package main

import (
	"errors"
	"net/url"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/metrics"
)

// configureInflux configures influxdb connection information
func configureInflux() (metrics.Client, error) {

	var metricClient metrics.Client

	// parse influxdb connection url
	influxURLParsed, err := url.Parse(cfg.InfluxURL)
	if err != nil {
		return metricClient, errors.New("Unable to parse influxUrl: " + err.Error())
	}

	// return an influx client with the right configuration details in it
	return metrics.NewInfluxClient(metrics.InfluxClientInput{
		Config: metrics.InfluxConfig{
			URL:      *influxURLParsed,
			Password: cfg.InfluxPassword,
			Username: cfg.InfluxUsername,
		},
		Database: cfg.InfluxDB,
	})
}
