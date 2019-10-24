package awsutil

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/sirupsen/logrus"
)

// CreateAWSSession creates and returns and AWS session.
func CreateAWSSession() *session.Session {
	// Build an AWs client.
	log.Infoln("Building AWS client.")
	return session.Must(session.NewSession(aws.NewConfig()))
}
