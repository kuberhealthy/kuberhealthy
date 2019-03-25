package metrics

// Metric is a key value struct
type Metric []map[string]interface{}

// Client is an abstraction for pushing metrics to custom providers
type Client interface {
	Push(points Metric, tags map[string]string) error
}
