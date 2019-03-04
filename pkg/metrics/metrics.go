package metrics

import "fmt"

// MetricClient is an abstraction for pushing metrics to custom providers
type MetricClient interface {
	Push(key string, value interface{}, tags map[string]string) error
}

// NewMetricClient returns a MetricClient that can be used to push Kuberhealthy
// metrics to a custom metrics provider
func NewMetricClient(input interface{}) (MetricClient, error) {
	switch input.(type) {
	case InfluxClientInput:
		influxInput, _ := input.(InfluxClientInput)
		return NewInfluxClient(influxInput)
	default:
		return nil, fmt.Errorf("Invalid metric client provided")
	}
}
