package awsutil

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/sirupsen/logrus"
)

// CreateAWSSession creates and returns an AWS session.
func CreateAWSSession() *session.Session {
	// Build an AWS session.
	log.Infoln("Building AWS session.")
	return session.Must(session.NewSession(aws.NewConfig().WithCredentialsChainVerboseErrors(true)))
}
