package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	log "github.com/sirupsen/logrus"
)

// createLambdaClient creates and returns an AWS Lambda client.
func createLambdaClient(s *session.Session, region string) *lambda.Lambda {
	// Build a Lambda client.
	log.Infoln("Building Lambda client for " + region + " region.")
	return lambda.New(s, &aws.Config{Region: aws.String(region)})
}

// listLambdas returns a list of Lambda functions.
func listLambdas(c *lambda.Lambda) ([]*lambda.FunctionConfiguration, error) {

	// Create an output object for the list query.
	list := make([]*lambda.FunctionConfiguration, 0)

	// Query Lambdas.
	log.Infoln("Querying Lambda functions.")
	results, err := c.ListFunctions(&lambda.ListFunctionsInput{
		MaxItems: aws.Int64(100),
	})
	if err != nil {
		return list, err
	}

	log.Infoln("Queried", len(results.Functions), "Lambdas.")
	list = append(list, results.Functions...)

	// Keep querying until the API gives us everything.
	var marker string
	for results.NextMarker != nil {
		log.Debugln("There are more results to be queried.")

		marker = *results.NextMarker
		results, err = c.ListFunctions(&lambda.ListFunctionsInput{
			MaxItems: aws.Int64(100),
			Marker:   aws.String(marker),
		})
		if err != nil {
			return list, err
		}

		log.Infoln("Queried", len(results.Functions), "Lambdas.")
		list = append(list, results.Functions...)
	}

	return list, nil
}

// runLambdaCheck runs the check. Returns a channel of errors and runs the check in the background.
// Lists Lambdas from AWS and sends an error through the channel if the query fails to return any Lambda
// configurations. If an expected Lambda count was given, this expects the number of Lambda
// configurations to match the given count and sends an error through the channel if the amounts
// mismatch. Otherwise if no expected Lambda count is given, being able to query ANY amount of Lambda
// configurations will send a nil down the channel.
func runLambdaCheck() chan error {

	// Create a channel for the check.
	checkChan := make(chan error, 0)

	log.Debugln("Starting Lambda check.")

	go func() {
		defer close(checkChan)

		// Create a Lambda client.
		c := createLambdaClient(sess, awsRegion)

		// Use the Lambda client to list of Lambda functions.
		list, err := listLambdas(c)
		if err != nil {
			checkChan <- err
			return
		}
		log.Infoln("Found", len(list), "Lambdas.")

		// If an expected count is given, we expect the number of Lambdas to be the same.
		if expectedLambdaCount != 0 {
			if len(list) != expectedLambdaCount {
				checkChan <- fmt.Errorf("mismatching count of Lambdas -- expected %d, but got %d", expectedLambdaCount, len(list))
				return
			}
			checkChan <- nil
			return
		}

		// Otherwise, if we could list ANY Lambda, the then we assume an OK.
		if len(list) != 0 {
			checkChan <- nil
			return
		}

		// If an expected count is given and no Lambdas were found, we assume a failure.
		checkChan <- fmt.Errorf("could not find any Lambdas")
		return
	}()

	return checkChan
}
