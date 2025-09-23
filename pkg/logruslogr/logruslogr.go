package logruslogr

import (
	"fmt"
	"runtime/debug"

	"github.com/go-logr/logr"
	log "github.com/sirupsen/logrus"
)

// New returns a logr.Logger that writes to the provided logrus.Logger.
func New(l *log.Logger) logr.Logger {
	return logr.New(&sink{logger: l})
}

type sink struct {
	logger *log.Logger
	name   string
	values []interface{}
}

// Init satisfies the logr.LogSink interface but logrus does not need runtime metadata.
func (s *sink) Init(info logr.RuntimeInfo) {}

// Enabled reports whether the requested log level should be emitted.
func (s *sink) Enabled(level int) bool {
	if level > 0 {
		return s.logger.IsLevelEnabled(log.DebugLevel)
	}
	return s.logger.IsLevelEnabled(log.InfoLevel)
}

// Info writes structured key/value data to logrus with the correct level.
func (s *sink) Info(level int, msg string, kv ...interface{}) {
	fields := s.collect(kv...)
	entry := s.logger.WithFields(fields)
	if s.name != "" {
		entry = entry.WithField("logger", s.name)
	}
	if s.name == "KubeAPIWarningLogger" {
		// Include stack trace to help locate where warnings originate.
		entry = entry.WithField("stack", string(debug.Stack()))
		entry.Warn(msg)
		return
	}
	if level > 0 {
		entry.Debug(msg)
		return
	}
	entry.Info(msg)
}

// Error writes error details and structured context to logrus.
func (s *sink) Error(err error, msg string, kv ...interface{}) {
	fields := s.collect(kv...)
	if err != nil {
		fields["error"] = err
	}
	entry := s.logger.WithFields(fields)
	if s.name != "" {
		entry = entry.WithField("logger", s.name)
	}
	entry.Error(msg)
}

// collect merges key/value pairs from the logger state and folds them into a logrus field map.
func (s *sink) collect(kv ...interface{}) log.Fields {
	out := log.Fields{}
	all := append(append([]interface{}{}, s.values...), kv...)
	for i := 0; i+1 < len(all); i += 2 {
		key, ok := all[i].(string)
		if !ok {
			key = fmt.Sprint(all[i])
		}
		out[key] = all[i+1]
	}
	return out
}

// WithValues returns a copy of the sink that remembers additional key/value pairs.
func (s *sink) WithValues(kv ...interface{}) logr.LogSink {
	ns := *s
	ns.values = append(append([]interface{}{}, s.values...), kv...)
	return &ns
}

// WithName returns a copy of the sink annotated with a hierarchical logger name.
func (s *sink) WithName(name string) logr.LogSink {
	ns := *s
	if s.name == "" {
		ns.name = name
		return &ns
	}

	ns.name = s.name + "/" + name
	return &ns
}
