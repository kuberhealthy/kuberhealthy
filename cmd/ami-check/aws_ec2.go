package main

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/ec2"
)

const (
	// We have to explicitly list account owners of the images that we want to look for.
	// Otherwise, AWS will give us a massive list that doesn't contain any of the commonly
	// used AMIs.

	// WellKnownAccountKopeio = kops account.
	WellKnownAccountKopeio = "383156758163"
	// WellKnownAccountRedhat = Red Hat account.
	WellKnownAccountRedhat = "309956199498"
	// WellKnownAccountCoreOS = CoreOS account.
	WellKnownAccountCoreOS = "595879546273"
	// WellKnownAccountAmazonSystemLinux2 = AWS Linux account.
	WellKnownAccountAmazonSystemLinux2 = "137112412989"
	// WellKnownAccountUbuntu = Ubuntu account.
	WellKnownAccountUbuntu = "099720109477"
)

// AMIResult struct represents a query for AWS AMIs. Contains a list
// of AMIs and an error.
type AMIResult struct {
	Images []*ec2.Image
	Err    error
}

// listEC2Images lists images available in AWS EC2.
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
