package main

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/lambda"
	log "github.com/sirupsen/logrus"
)

// createLambdaClient creates and returns an AWS Lambda client
func createLambdaClient(s *session.Session, region string) *lambda.Lambda {

	// build a lambda client
	log.Infoln("Building Lambda client.")
	c := lambda.New(s, &aws.Config{Region: aws.String(region)})

	return c
}

// listLambdas returns a list of Lambda functions.
func listLambdas(c *lambda.Lambda) ([]*lambda.FunctionConfiguration, error) {

	// Create an output object for the list query.
	var marker string
	list := make([]*lambda.FunctionConfiguration, 0)

	// Query Lambdas.
	log.Infoln("Querying lambda functions.")
	for {
		results, err := c.ListFunctions(&lambda.ListFunctionsInput{
			MaxItems: aws.Int64(1000),
			Marker:   aws.String(marker),
		})
		if err != nil {
			return list, err
		}
		list = append(list, results.Functions...)

		if results.NextMarker == nil {
			break
		}
		marker = *results.NextMarker
	}
	return list, nil
}

func runLambdaCheck() chan error {

	checkChan := make(chan error, 0)

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

		// If not expected count is given and no Lambdas were found, we assume a failure.
		checkChan <- fmt.Errorf("could not find any Lambdas")
		return
	}()

	return checkChan
}
