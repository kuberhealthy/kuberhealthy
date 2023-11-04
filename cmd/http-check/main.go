package main

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
)

var (
	// Environment Variables fetched from spec file
	checkURL           = os.Getenv("CHECK_URL")
	count              = os.Getenv("COUNT")
	seconds            = os.Getenv("SECONDS")
	passing            = os.Getenv("PASSING_PERCENT")
	requestType        = os.Getenv("REQUEST_TYPE")
	requestBody        = os.Getenv("REQUEST_BODY")
	expectedStatusCode = os.Getenv("EXPECTED_STATUS_CODE")
)

type APIRequest struct {
	URL  *url.URL
	Type string
	Body io.Reader
}

func init() {
	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()

	// Check that the URL environment variable is valid.
	if len(checkURL) == 0 {
		err := fmt.Errorf("empty CHECK_URL specified. Please update your CHECK_URL environment variable")
		ReportFailureAndExit(err)
	}

	// Check that the COUNT environment variable is valid.
	if len(count) == 0 {
		count = "0"
	}

	// Check that the SECONDS environment variable is valid.
	if len(seconds) == 0 {
		seconds = "0"
	}

	// Check that the PASSING_PERCENT environment variable is valid.
	if len(passing) == 0 {
		passing = "100" //or 80 or whatever the default passing percent value
	}

	// Check that the REQUEST_TYPE environment variable is valid.
	if len(requestType) == 0 {
		requestType = "GET"
	}

	// Check that the REQUEST_BODY environment variable is valid.
	if len(requestBody) == 0 {
		requestBody = "{}"
	}

	// Check that the EXPECTED_STATUS_CODE environment variable is valid.
	if len(expectedStatusCode) == 0 {
		expectedStatusCode = "200"
	}

	// If the URL does not begin with HTTP, exit.
	if !strings.HasPrefix(checkURL, "http") {
		err := fmt.Errorf("given URL does not declare a supported protocol. (http | https)")
		ReportFailureAndExit(err)
	}
}

func main() {
	// create context
	checkTimeLimit := time.Minute * 1
	ctx, _ := context.WithTimeout(context.Background(), checkTimeLimit)

	// validate url
	parsedUrl, err := url.Parse(checkURL)
	if err != nil {
		log.Errorln("Cannot parse provided URL:" + err.Error())
		ReportFailureAndExit(err)
	}

	// hits kuberhealthy endpoint to see if node is ready
	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	countInt, err := strconv.Atoi(count)
	if err != nil {
		err = fmt.Errorf("Error converting COUNT to int: " + err.Error())
		ReportFailureAndExit(err)
	}

	secondInt, err := strconv.Atoi(seconds)
	if err != nil {
		err = fmt.Errorf("Error converting SECONDS to int: " + err.Error())
		ReportFailureAndExit(err)
	}

	passingInt, err := strconv.Atoi(passing)
	if err != nil {
		err = fmt.Errorf("Error converting PASSING_PERCENT to int: " + err.Error())
		ReportFailureAndExit(err)
	}

	expectedStatusCodeInt, err := strconv.Atoi(expectedStatusCode)
	if err != nil {
		err = fmt.Errorf("Error converting EXPECTED_STATUS_CODE to int: " + err.Error())
		ReportFailureAndExit(err)
	}

	// if the expected status code is empty, then default to 200
	if expectedStatusCodeInt == 0 {
		expectedStatusCodeInt = 200
	}

	// if the passing count is empty, then default to 100%
	if passingInt == 0 {
		passingInt = 100
	}
	passingPercentage := float32(passingInt) / 100

	// sets the passing score to compare it against checks that have been ran
	passingScore := passingPercentage * float32(countInt)
	passInt := int(passingScore)
	log.Infoln("Looking for at least", passingInt, "percent of", countInt, "checks to pass")

	// init counters for checks
	log.Infoln("Beginning check.")
	checksRan := 0
	checksPassed := 0
	checksFailed := 0

	// if we have a pause, start a ticker
	var ticker *time.Ticker
	if secondInt > 0 {
		ticker = time.NewTicker(time.Duration(secondInt) * time.Second)
		defer ticker.Stop()
	}

	// This for loop makes a http GET request to a known internet address, address can be changed in deployment spec yaml
	// and returns a http status every second.
	for checksRan < countInt {
		r, err := callAPI(APIRequest{
			URL:  parsedUrl,
			Type: requestType,
			Body: bytes.NewBuffer([]byte(requestBody)),
		})
		checksRan++

		if err != nil {
			checksFailed++
			log.Errorln("Failed to reach URL: ", parsedUrl.Redacted())
			continue
		}

		if r.StatusCode != expectedStatusCodeInt {
			log.Errorln("Got a", r.StatusCode, "with a", http.MethodGet, "to", parsedUrl.Redacted())
			checksFailed++
			continue
		}
		log.Infoln("Got a", r.StatusCode, "with a", http.MethodGet, "to", parsedUrl.Redacted())
		checksPassed++

		// if we have a ticker, we wait for it to tick before checking again
		if ticker != nil && ticker.C != nil {
			<-ticker.C
		}
	}

	// Displays the results of 10 URL requests
	log.Infoln(checksRan, "checks ran")
	log.Infoln(checksPassed, "checks passed")
	log.Infoln(checksFailed, "checks failed")

	// Check to see if the number of requests passed at passingPercent and reports to Kuberhealthy accordingly
	if checksPassed < passInt {
		reportErr := fmt.Errorf("unable to retrieve a valid response (expected status: %d) from %s %s checks failed %d out of %d attempts", expectedStatusCodeInt, requestType, parsedUrl.Redacted(), checksFailed, checksRan)
		ReportFailureAndExit(reportErr)
	}

	err = kh.ReportSuccess()
	if err != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	log.Infoln("Successfully reported to Kuberhealthy")
}

// ReportFailureAndExit logs and reports an error to kuberhealthy and then exits the program.
// If a error occurs when reporting to kuberhealthy, the program fatals.
func ReportFailureAndExit(err error) {
	log.Errorln(err)
	err2 := kh.ReportFailure([]string{err.Error()})
	if err2 != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	os.Exit(0)
}

// callAPI performs an API call on the basis of the request type, body and URL provided to it.
// It returns the response corresponding to the request.
func callAPI(request APIRequest) (*http.Response, error) {
	var response *http.Response
	switch request.Type {
	case "GET":
		resp, err := http.Get(request.URL.String())
		if err != nil {
			return nil, fmt.Errorf("error occurred while calling %s: %w", request.URL.Redacted(), err)
		}
		response = resp
	case "POST", "PUT", "DELETE", "PATCH":
		req, err := http.NewRequest(request.Type, request.URL.String(), request.Body)
		if err != nil {
			return nil, fmt.Errorf("error occurred while calling %s: %w", request.URL.Redacted(), err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("error occurred while calling %s: %w", request.URL.Redacted(), err)
		}
		response = resp
	default:
		return nil, fmt.Errorf("error occurred while calling %s: wrong request type found", request.URL.Redacted())
	}

	return response, nil
}
