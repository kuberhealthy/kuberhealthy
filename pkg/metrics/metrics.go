package metrics

// Client is an abstraction for pushing metrics to custom providers
type Client interface {
	Push(key string, value interface{}, tags map[string]string) error
}
