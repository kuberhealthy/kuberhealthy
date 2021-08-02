// Package dnsStatus implements a DNS checker for Kuberhealthy
// It verifies that local DNS and external DNS are functioning correctly
package main

import (
	"context"
	"errors"
	"net"
	"os"
	"path/filepath"
	"time"

	log "github.com/sirupsen/logrus"
	// required for oidc kubectl testing
	v1 "k8s.io/api/core/v1"
	_ "k8s.io/client-go/plugin/pkg/client/auth/oidc"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"

	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
)

const maxTimeInFailure = 60 * time.Second
const defaultCheckTimeout = 5 * time.Minute

// KubeConfigFile is a variable containing file path of Kubernetes config files
var KubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// CheckTimeout is a variable for how long code should run before it should retry
var CheckTimeout time.Duration

// Hostname is a variable for container/pod name
var Hostname string

// NodeName is a variable for the node where the container/pod is created
var NodeName string

// Namespace where dns pods live
var namespace string

// Label selector used for dns pods
var labelSelector string

var now time.Time

// Checker validates that DNS is functioning correctly
type Checker struct {
	client           *kubernetes.Clientset
	MaxTimeInFailure time.Duration
	Hostname         string
}

func init() {

	// Set check time limit to default
	CheckTimeout = defaultCheckTimeout
	// Get the deadline time in unix from the env var
	timeDeadline, err := checkclient.GetDeadline()
	if err != nil {
		log.Infoln("There was an issue getting the check deadline:", err.Error())
	}
	CheckTimeout = timeDeadline.Sub(time.Now().Add(time.Second * 5))
	log.Infoln("Check time limit set to:", CheckTimeout)

	Hostname = os.Getenv("HOSTNAME")
	if len(Hostname) == 0 {
		log.Errorln("ERROR: The ENDPOINT environment variable has not been set.")
		return
	}

	NodeName = os.Getenv("NODE_NAME")
	if len(NodeName) == 0 {
		log.Errorln("ERROR: Failed to retrieve NODE_NAME environment variable.")
		return
	}
	log.Infoln("Check pod is running on node:", NodeName)

	namespace = os.Getenv("NAMESPACE")
	if len(namespace) > 0 {
		log.Infoln("Looking for DNS pods in namespace:", namespace)
	}

	labelSelector = os.Getenv("DNS_POD_SELECTOR")
	if len(labelSelector) > 0 {
		log.Infoln("Looking for DNS pods with label:", labelSelector)
	}

	now = time.Now()
}

func main() {
	client, err := kubeClient.Create(KubeConfigFile)
	if err != nil {
		log.Fatalln("Unable to create kubernetes client", err)
	}

	dc := New()

	// Since this check runs often and very quickly, run all nodeChecks to make sure that:
	// - check doesn't run on a node that's too young
	// - kuberhealthy endpoint is ready
	// - kube-proxy is ready and running on the node the check is running on
	nodeCheck.EnableDebugOutput()
	checkTimeLimit := time.Minute * 1
	ctx, _ := context.WithTimeout(context.Background(), checkTimeLimit)

	minNodeAge := time.Minute * 3
	err = nodeCheck.WaitForNodeAge(ctx, client, NodeName, minNodeAge)
	if err != nil {
		log.Errorln("Error waiting for node to reach minimum age:", err)
	}

	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for Kuberhealthy to be ready:", err)
	}

	err = dc.Run(client)
	if err != nil {
		log.Errorln("Error running DNS Status check for hostname:", Hostname)
	}
	log.Infoln("Done running DNS Status check for hostname:", Hostname)
}

// New returns a new DNS Checker
func New() *Checker {
	return &Checker{
		Hostname:         Hostname,
		MaxTimeInFailure: maxTimeInFailure,
	}
}

// Run implements the entrypoint for check execution
func (dc *Checker) Run(client *kubernetes.Clientset) error {
	log.Infoln("Running DNS status checker")
	doneChan := make(chan error)

	dc.client = client
	// run the check in a goroutine and notify the doneChan when completed
	go func(doneChan chan error) {
		err := dc.doChecks()
		doneChan <- err
	}(doneChan)

	// wait for either a timeout or job completion
	select {
	case <-time.After(CheckTimeout):
		// The check has timed out after its specified timeout period
		errorMessage := "Failed to complete DNS Status check in time! Timeout was reached."
		err := checkclient.ReportFailure([]string{errorMessage})
		if err != nil {
			log.Println("Error reporting failure to Kuberhealthy servers:", err)
			return err
		}
		return err
	case err := <-doneChan:
		if err != nil {
			return reportKHFailure(err.Error())
		}
		return reportKHSuccess()
	}
}

// create a Resolver object to use for DNS queries
func createResolver(ip string) (*net.Resolver, error) {
	r := &net.Resolver{}
	// if we're supplied a null string, return an error
	if len(ip) < 1 {
		return r, errors.New("Need a valid ip to create Resolver")
	}
	// attempt to create the resolver based on the string
	r = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address2 string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, "udp", ip+":53")
		},
	}
	return r, nil
}

func getIpsFromEndpoint(endpoints *v1.EndpointsList) ([]string, error) {
	var ipList []string
	if len(endpoints.Items) == 0 {
		return ipList, errors.New("No endpoints found")
	}
	// loop through endpoint list, subsets, and finally addresses to get backend DNS ip's to query
	for ep := 0; ep < len(endpoints.Items); ep++ {
		for sub := 0; sub < len(endpoints.Items[ep].Subsets); sub++ {
			for address := 0; address < len(endpoints.Items[ep].Subsets[sub].Addresses); address++ {
				// create resolver based on backends found in the dns endpoint
				ipList = append(ipList, endpoints.Items[ep].Subsets[sub].Addresses[address].IP)
			}
		}
	}
	if len(ipList) != 0 {
		return ipList, nil
	}
	return ipList, errors.New("No Ip's found in endpoints list")
}

func dnsLookup(r *net.Resolver, host string) error {
	_, err := r.LookupHost(context.Background(), host)
	if err != nil {
		errorMessage := "DNS Status check determined that " + host + " is DOWN: " + err.Error()
		return errors.New(errorMessage)
	}
	return nil
}

func (dc *Checker) checkEndpoints() error {
	endpoints, err := dc.client.CoreV1().Endpoints(namespace).List(context.Background(), metav1.ListOptions{LabelSelector: labelSelector})
	if err != nil {
		message := "DNS status check unable to get dns endpoints from cluster: " + err.Error()
		log.Errorln(message)
		return errors.New(message)
	}

	//get ips from endpoint list to check
	ips, err := getIpsFromEndpoint(endpoints)
	if err != nil {
		return err
	}

	//given that we got valid ips from endpoint list, parse them
	if len(ips) > 0 {
		for ip := 0; ip < len(ips); ip++ {
			//create a resolver for each ip and return any error
			r, err := createResolver(ips[ip])
			if err != nil {
				return err
			}
			//run a lookup for each ip if we successfully created a resolver, return error
			err = dnsLookup(r, dc.Hostname)
			if err != nil {
				return err
			}
		}
		log.Infoln("DNS Status check from service endpoint determined that", dc.Hostname, "was OK.")
		return nil
	}
	return errors.New("No ips found in endpoint with label: " + labelSelector)
}

// doChecks does validations on the DNS call to the endpoint
func (dc *Checker) doChecks() error {

	log.Infoln("DNS Status check testing hostname:", dc.Hostname)

	// if there's a label selector, do checks against endpoints
	if len(labelSelector) > 0 {
		err := dc.checkEndpoints()
		if err != nil {
			return err
		}
		return nil
	}

	// otherwise do lookup against service endpoint
	_, err := net.LookupHost(dc.Hostname)
	if err != nil {
		errorMessage := "DNS Status check determined that " + dc.Hostname + " is DOWN: " + err.Error()
		log.Errorln(errorMessage)
		return errors.New(errorMessage)
	}
	log.Infoln("DNS Status check from service endpoint determined that", dc.Hostname, "was OK.")
	return nil
}

// reportKHSuccess reports success to Kuberhealthy servers and verifies the report successfully went through
func reportKHSuccess() error {
	err := checkclient.ReportSuccess()
	if err != nil {
		log.Println("Error reporting success to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported success to Kuberhealthy servers")
	return err
}

// reportKHFailure reports failure to Kuberhealthy servers and verifies the report successfully went through
func reportKHFailure(errorMessage string) error {
	err := checkclient.ReportFailure([]string{errorMessage})
	if err != nil {
		log.Println("Error reporting failure to Kuberhealthy servers:", err)
		return err
	}
	log.Println("Successfully reported failure to Kuberhealthy servers")
	return err
}
