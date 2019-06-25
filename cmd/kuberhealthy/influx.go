package main

import (
	"errors"
	"net/url"

	"github.com/Comcast/kuberhealthy/pkg/metrics"
)

// configureInflux configures influxdb connection information
func configureInflux() (metrics.Client, error) {

	var metricClient metrics.Client

	// parse influxdb connection url
	influxUrlParsed, err := url.Parse(influxUrl)
	if err != nil {
		return metricClient, errors.New("Unable to parse influxUrl: " + err.Error())
	}

	// return an influx client with the right configuration details in it
	return metrics.NewInfluxClient(metrics.InfluxClientInput{
		Config: metrics.InfluxConfig{
			URL:      *influxUrlParsed,
			Password: influxPassword,
			Username: influxUsername,
		},
		Database: influxDB,
	})
}
