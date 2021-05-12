package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gorhill/cronexpr"
	kh "github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/checkclient"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/checks/external/nodeCheck"
	"github.com/kuberhealthy/kuberhealthy/v2/pkg/kubeClient"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// kubeConfigFile is a variable containing file path of Kubernetes config files
var kubeConfigFile = filepath.Join(os.Getenv("HOME"), ".kube", "config")

// namespace is a variable to allow code to target all namespaces or a single namespace
var namespace = os.Getenv("NAMESPACE")

// Check deadline from injected env variable KH_CHECK_RUN_DEADLINE
var khCheckRunDeadlineEnv = os.Getenv("KH_CHECK_RUN_DEADLINE")

func init() {
	// set debug mode for nodeCheck pkg
	nodeCheck.EnableDebugOutput()
}

func main() {

	// create check run deadline
	intkhCheckRundDeadline, err := strconv.ParseInt(khCheckRunDeadlineEnv, 10, 64)
	if err != nil {
		log.Fatalln("Error parsing KH_CHECK_RUN_DEADLINE:", err)
	}
	khCheckRunDeadline := time.Unix(intkhCheckRundDeadline, 0)

	khCheckRunTime := khCheckRunDeadline.Sub(time.Now()) / 2

	ctx, _ := context.WithTimeout(context.Background(), khCheckRunTime)

	// create kubeClient
	client, err := kubeClient.Create(kubeConfigFile)
	if err != nil {
		errorMessage := fmt.Errorf("Failed to create a kubernetes client with error: " + err.Error())
		ReportFailureAndExit(errorMessage)
		return
	}

	// hits kuberhealthy endpoint to see if node is ready
	err = nodeCheck.WaitForKuberhealthy(ctx)
	if err != nil {
		log.Errorln("Error waiting for kuberhealthy endpoint to be contactable by checker pod with error:" + err.Error())
	}

	log.Infoln("Fetching cronjobs in namespace", namespace)

	// create cronjob client from kubeClient
	cronList, err := client.BatchV1beta1().CronJobs(namespace).List(ctx, v1.ListOptions{})
	if err != nil {
		log.Errorln("Failed to fetch cronjobs with error:", err)
	}

	log.Infoln("Found", len(cronList.Items), "cronjob(s) in namespace", namespace)

	probCount := 0
	goodCount := 0

	// range over cronjobs
	for _, c := range cronList.Items {

		// continue to next cronjob if it is new and has not scheduled yet
		if c.Status.LastScheduleTime == nil {
			continue
		}

		// create client to gather information for specific cronjob
		cronGet, err := client.BatchV1beta1().CronJobs(namespace).Get(ctx, c.Name, v1.GetOptions{})
		if err != nil {
			log.Errorln("Error retrieving cronjob status for cronjob", c.Name, "with error:", err)
			continue
		}

		// schedule information for cronjob
		// creationTimeStamp := c.CreationTimestamp.Time
		schedule := cronGet.Spec.Schedule
		lastRunTimeV1 := cronGet.Status.LastScheduleTime
		lastRunTime := lastRunTimeV1.Time
		shouldOfRun := findLastCronRunTime(schedule)
		earliestRunTime, latestRunTime := scheduleWindow(shouldOfRun, time.Minute*10)

		log.Infoln("Cronjob", c.Name, "was last scheduled at", lastRunTime)

		if lastRunTime.After(earliestRunTime) && lastRunTime.Before(latestRunTime) {
			log.Infoln("Cronjob " + c.Name + " is scheduling correctly")
			goodCount++
			continue
		}
		log.Infoln("Cronjob " + c.Name + " has not scheduled a job in scheduled window. Please confirm there are no issues with cronjob in namespace " + c.Namespace)
		probCount++
	}

	// report issues to kuberhealthy if any are found
	if probCount != 0 {
		log.Infoln("There were " + strconv.Itoa(probCount) + " cronjob(s) that had a last schedule time outside of scheduled window in namespace " + namespace)
		reportErr := fmt.Errorf(("There were " + strconv.Itoa(probCount) + " cronjob(s) that had a last schedule time outside of scheduled window in namespace " + namespace))
		ReportFailureAndExit(reportErr)
	}

	log.Infoln("All cronjobs in namespace " + namespace + " scheduled jobs in schedule window")

	// report success to kuberhealthy
	err = kh.ReportSuccess()
	if err != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	log.Infoln("Successfully reported to Kuberhealthy")

}

func findLastCronRunTime(schedule string) time.Time {
	cronSchedule := cronexpr.MustParse(schedule) // the cron schedule of the check
	oneYear := time.Hour * 24 * 366
	oneYearAgo := time.Now().Add(-oneYear)
	now := time.Now()
	timeMarker := oneYearAgo // the time marker we will walk forward until we pass ourselves with the next run time prediction
	for {
		nextRunTime := cronSchedule.Next(timeMarker)
		// if the next forecast run time is after right now, then we stop and return that time (it is the last time that should have ran)
		if nextRunTime.After(now) {
			return timeMarker
		}
		timeMarker = nextRunTime
	}
}

// ReportFailureAndExit logs and reports an error to kuberhealthy and then exits the program.
// If a error occurs when reporting to kuberhealthy, the program fatals.
func ReportFailureAndExit(err error) {
	// log.Errorln(err)
	err2 := kh.ReportFailure([]string{err.Error()})
	if err2 != nil {
		log.Fatalln("error when reporting to kuberhealthy:", err.Error())
	}
	log.Infoln("Succesfully reported error to kuberhealthy")
	os.Exit(0)
}

// scheduleWindow
func scheduleWindow(t time.Time, d time.Duration) (time.Time, time.Time) {

	timeMinus := t.Add(-1 * (d / 2))
	timePlus := t.Add(d / 2)

	return timeMinus, timePlus
}
