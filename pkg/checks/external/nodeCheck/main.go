package nodeCheck

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"os"
	"time"

	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external"
)

// EnableDebugOutput enables debug logging for this package
func EnableDebugOutput() {
	log.SetLevel(log.DebugLevel)
}

// WaitForKuberhealthy waits for the the kuberhealthy endpoint (KH_REPORTING_URL) to be contactable by the checker pod
// on a given node
func WaitForKuberhealthy(ctx context.Context) error {

	KHReportingURL, err := url.Parse(os.Getenv(external.KHReportingURL))
	// check the length of the reporting url to make sure we pulled one properly
	if err != nil {
		return errors.New("error parsing kuberhealthy reporting URL from environment variable " +
			external.KHReportingURL)
	}
	// Point endpoint check to the healthcheck path (/), instead of reporting path as it expects check status payload
	KHReportingURL.Path = "/"
	kuberhealthyEndpoint := KHReportingURL.String()

	log.Debugln("Checking if the kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready.")
	select {
	case err := <-waitForKuberhealthyEndpointReady(ctx, kuberhealthyEndpoint):
		if err != nil {
			return err
		}
		log.Debugln("Kuberhealthy endpoint:", kuberhealthyEndpoint, "is ready. Proceeding to run check.")
	case <-ctx.Done():
		return errors.New("context cancelled waiting for Kuberhealthy endpoint to be ready")
	}
	return nil
}

// WaitForNodeAge checks the node's age to see if its less than the minimum node age. If so, sleeps until the node
// reaches the minimum node age.
func WaitForNodeAge(ctx context.Context, client *kubernetes.Clientset, nodeName string, minNodeAge time.Duration) error {

	log.Debugln("Pod is on node:", nodeName)

	node, err := client.CoreV1().Nodes().Get(ctx, nodeName, v1.GetOptions{})
	if err != nil {
		return err
	}
	// get current age of the node
	nodeAge := time.Now().Sub(node.CreationTimestamp.Time)
	log.Debugln("Check running on node: ", node.Name, "with node age:", nodeAge)
	if nodeAge >= minNodeAge {
		return nil
	}

	select {
	case <-ctx.Done():
		return errors.New("context cancelled waiting for node to reach minNodeAge")
	default:
		sleepDuration := minNodeAge - nodeAge
		log.Debugln("Node is new. Sleeping for:", sleepDuration, "until node reaches minNodeAge:", minNodeAge)
		time.Sleep(sleepDuration)
	}
	return nil
}

// waitForKuberhealthyEndpointReady hits the kuberhealthy endpoint every 3 seconds to see if the node is ready to reach
// the endpoint.
func waitForKuberhealthyEndpointReady(ctx context.Context, kuberhealthyEndpoint string) chan error {

	doneChan := make(chan error, 1)

	for {
		select {
		case <-ctx.Done():
			doneChan <- errors.New("context cancelled waiting for Kuberhealthy endpoint to be ready")
			return doneChan
		default:
		}
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(kuberhealthyEndpoint)
		if err == nil {
			if resp.StatusCode == http.StatusOK {
				log.Debugln(kuberhealthyEndpoint, "is ready.")
				doneChan <- nil
				return doneChan
			} else {
				log.Debugf("Endpoint %s is not ready yet... resp code: %d", kuberhealthyEndpoint, resp.StatusCode)
			}
		} else {
			log.Debugf("Endpoint %s is not responding yet... err: %s", kuberhealthyEndpoint, err.Error())
		}
		time.Sleep(time.Second * 3)
	}
}
