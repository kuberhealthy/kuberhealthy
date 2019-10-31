package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

// AMIResult struct represents a query for AWS AMIs. Contains a list
// of AMIs and an error.
type AMIResult struct {
	Images []*ec2.Image
	Err    error
}

const (
	WellKnownAccountKopeio             = "383156758163"
	WellKnownAccountRedhat             = "309956199498"
	WellKnownAccountCoreOS             = "595879546273"
	WellKnownAccountAmazonSystemLinux2 = "137112412989"
	WellKnownAccountUbuntu             = "099720109477"
)

// listEC2Images lists images available in AWS EC2
func listEC2Images() chan AMIResult {

	listChan := make(chan AMIResult)

	go func() {
		defer close(listChan)

		awsEC2 := ec2.New(awsSess, &aws.Config{Region: aws.String(awsRegion)})

		amiResult := AMIResult{}

		kopeioOwner := WellKnownAccountKopeio
		redHatOwner := WellKnownAccountRedhat
		coreOSOwner := WellKnownAccountCoreOS
		awsLinux2Owner := WellKnownAccountAmazonSystemLinux2
		// ubuntuOwner := WellKnownAccountUbuntu

		// Get a list of images from trusted owners.
		images, err := awsEC2.DescribeImages(&ec2.DescribeImagesInput{
			Owners: []*string{
				&kopeioOwner,
				&redHatOwner,
				&coreOSOwner,
				&awsLinux2Owner,
				// &ubuntuOwner,
			},
		})
		if err != nil {
			amiResult.Err = err
			listChan <- amiResult
			return
		}

		amiResult.Images = images.Images
		listChan <- amiResult
		return
	}()

	return listChan
}
