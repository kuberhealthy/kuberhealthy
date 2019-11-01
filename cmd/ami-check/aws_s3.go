package main

import (
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	log "github.com/sirupsen/logrus"
	"k8s.io/kops/pkg/apis/kops"
)

// KOPSResult struct represents a query for KOPS instance groups. Contains a list
// of instance groups and an error.
type KOPSResult struct {
	InstanceGroups []*kops.InstanceGroup
	Err            error
}

// listKopsInstanceGroups lists instance groups used by kops (stored in S3).
func listKopsInstanceGroups() chan KOPSResult {
	log.Infoln("Listing KOPS instance groups from AWS S3.")

	listChan := make(chan KOPSResult)

	go func() {
		defer close(listChan)

		kopsResult := KOPSResult{}

		// Make an S3 session.
		awsS3 := s3.New(awsSess, &aws.Config{Region: aws.String(awsRegion)})
		if awsS3 == nil {
			err := fmt.Errorf("nil S3 session: %v", awsS3)
			kopsResult.Err = err
			listChan <- kopsResult
			return
		}

		// List the objects in the bucket.
		objects, err := getObjects(awsS3)
		if err != nil {
			kopsResult.Err = err
			listChan <- kopsResult
			return
		}

		// Get the contents of instancegroup objects.
		igs, err := getObjectContents(awsS3, objects)
		if err != nil {
			kopsResult.Err = err
			listChan <- kopsResult
			return
		}

		log.Infoln("Found", len(igs), "instance groups.")
		kopsResult.InstanceGroups = igs
		listChan <- kopsResult
		return
	}()

	return listChan
}

// getObjects lists objects representing kops instance groups (stored in S3).
func getObjects(awsS3 *s3.S3) ([]*s3.Object, error) {
	var marker string
	log.Infoln("Querying object keys from S3 bucket.")
	results := make([]*s3.Object, 0)

	// List the objects within the bucket.
	// The first call does not need a marker.
	objects, err := awsS3.ListObjects(&s3.ListObjectsInput{
		Bucket: aws.String(awsS3BucketName),
	})
	if err != nil {
		log.Errorln("failed to list bucket objects:", err.Error())
		return results, err
	}

	results = append(results, objects.Contents...)

	// Keep querying for objects if there are more.
	for objects.Marker != nil {
		marker = *objects.Marker
		if len(marker) == 0 {
			break
		}

		log.Infoln("There are more bucket objects to be queried:", marker)

		objects, err = awsS3.ListObjects(&s3.ListObjectsInput{
			Bucket: aws.String(awsS3BucketName),
			Marker: aws.String(marker),
		})

		if err != nil {
			log.Errorln("failed to list bucket objects:", err.Error())
			return results, err
		}

		results = append(results, objects.Contents...)
	}

	log.Infoln("Found", len(results), "objects in this bucket.")
	return results, nil
}

// getObjectContents downloads and unmarshals S3 kops instance group objects into a
// slice of instance group structs.
func getObjectContents(awsS3 *s3.S3, objects []*s3.Object) ([]*kops.InstanceGroup, error) {
	log.Infoln("Reading S3 object contents.")

	results := make([]*kops.InstanceGroup, 0)

	for _, object := range objects {
		// Check to see if the object key matches "instancegroup" regexp. This will filter out the
		// files that do not represent a kops instancegroup.
		ok, err := regexp.Match(regexpKopsStateStoreS3ObjectKey, []byte(*object.Key))
		if err != nil {
			err = fmt.Errorf("failed to match object key with instancegroup regexp: %w", err)
			log.Errorln(err.Error())
			continue
		}
		if !ok {
			log.Debugln("Skipping object with key:", *object.Key)
			continue
		}

		if !strings.Contains(*object.Key, clusterName) {
			log.Debugf("Skipping object due to mismatching cluster names. Object for %s, but looking for %s.\n", *object.Key, clusterName)
			continue
		}

		log.Infoln("Information for object with key:", *object.Key)

		// Make a request for the object.
		output, err := awsS3.GetObject(&s3.GetObjectInput{
			Key:    aws.String(*object.Key),
			Bucket: aws.String(awsS3BucketName),
		})
		if err != nil {
			log.Errorf("failed to list bucket object with key %s: %s", *object.Key, err.Error())
			return results, err
		}

		// Try to decode the object response body into a byte slice.
		objectBytes, err := ioutil.ReadAll(output.Body)
		if err != nil {
			log.Errorf("failed to read object body: %s", err.Error())
			log.Infoln("Skipping", *object.Key)
			continue
		}

		// Parse the data into the instance group struct.
		var ig kops.InstanceGroup
		err = kops.ParseRawYaml(objectBytes, &ig)
		if err != nil {
			err = fmt.Errorf("failed to unmarshal yaml data: %w", err)
			log.Errorln(err)
			continue
		}

		// Append the instance group if unmarshalling was ok.
		log.Infoln("Found and unmarshalled data for:", ig.Name)
		results = append(results, &ig)
	}

	return results, nil
}
