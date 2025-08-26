package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	yaml "github.com/ghodss/yaml"
	"github.com/google/uuid"
	kuberhealthycheckv2 "github.com/kuberhealthy/crds/api/v2"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/envs"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/metrics"
	log "github.com/sirupsen/logrus"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	client "sigs.k8s.io/controller-runtime/pkg/client"
)

const statusPageHTML = `
<!DOCTYPE html>
<html data-bs-theme="dark">
<head>
<meta charset="utf-8" />
<title>Kuberhealthy Status</title>
<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/css/bootstrap.min.css"/>
<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.2/dist/js/bootstrap.bundle.min.js"></script>
<style>
body {font-family:system-ui,sans-serif; margin:0; display:flex; flex-direction:column; height:100vh;}
#main {flex:1; display:flex; overflow:hidden;}
#menu {width:260px; border-right:1px solid var(--bs-border-color); overflow-y:auto;}
#menu .item {padding:10px; cursor:pointer;}
#menu .item:hover {background:var(--bs-secondary-bg);}
#content {flex:1; padding:1rem; overflow-y:auto;}
#pods .pod {cursor:pointer; padding:4px;}
#pods .pod:hover {background:var(--bs-secondary-bg);}
#events .event {padding:4px; border-bottom:1px solid var(--bs-border-color);}
pre {background:var(--bs-tertiary-bg); padding:1em; white-space:pre-wrap;}
</style>
<script>
let checks = {};
let currentCheck = '';

function setTheme(t){
  document.documentElement.setAttribute('data-bs-theme', t);
  localStorage.setItem('kh-theme', t);
  document.getElementById('themeToggle').textContent = t === 'dark' ? '☀️' : '🌙';
}

function initTheme(){
  const saved = localStorage.getItem('kh-theme') || 'dark';
  setTheme(saved);
}

function formatDuration(ms){
  const total = Math.max(0, Math.floor(ms/1000));
  const h = Math.floor(total/3600);
  const m = Math.floor((total%3600)/60);
  const s = total%60;
  if(h) return h+'h '+m+'m '+s+'s';
  if(m) return m+'m '+s+'s';
  return s+'s';
}

async function refresh(){
  try{
    const resp = await fetch('/json');
    const data = await resp.json();
    checks = data.CheckDetails || {};
    const menu = document.getElementById('menu');
    menu.innerHTML = '';
    const now = Date.now();
    Object.keys(checks).forEach(function(name){
      const st = checks[name];
      let icon = st.ok ? '✅' : '❌';
      if (st.podName){ icon = '⏳'; }
      const div = document.createElement('div');
      div.className = 'item';
      let label = icon + ' ' + name;
      if (st.nextRunUnix){
        const diff = st.nextRunUnix*1000 - now;
        label += ' (' + formatDuration(diff) + ')';
      }
      div.textContent = label;
      div.onclick = ()=>showCheck(name);
      menu.appendChild(div);
    });
  }catch(e){ console.error('failed to fetch status', e); }
}

async function showCheck(name){
  currentCheck = name;
  const st = checks[name];
  if(!st){return;}
  const content = document.getElementById('content');
  let nextRun = '';
  if(st.nextRunUnix){
    nextRun = formatDuration(st.nextRunUnix*1000 - Date.now());
  }
  content.innerHTML='<h2>'+name+'</h2>'
    + '<p>Status: '+(st.ok?'OK':'Fail')+'</p>'
    + (nextRun?'<p>Next run in: '+nextRun+'</p>':'')
    + (st.errors && st.errors.length ? '<p>Errors: '+st.errors.join('; ')+'</p>' : '')
    + '<h3>Events</h3><div id="events">loading...</div>'
    + '<h3>Pods</h3><div id="pods">loading...</div>'
    + '<h3>Pod Details</h3><div id="pod-info"></div>'
    + '<h3>Logs</h3><pre id="logs"></pre>';
  try{
    const pods = await (await fetch('/api/pods?namespace='+encodeURIComponent(st.namespace)+'&khcheck='+encodeURIComponent(name))).json();
    const podsDiv = document.getElementById('pods');
    podsDiv.innerHTML='';
    pods.forEach(p=>{
      const div=document.createElement('div');
      div.className='pod';
      div.textContent=p.name+' ('+p.phase+')';
      div.onclick=()=>loadLogs(p);
      podsDiv.appendChild(div);
    });
    try{
      const evs = await (await fetch('/api/events?namespace='+encodeURIComponent(st.namespace)+'&khcheck='+encodeURIComponent(name))).json();
      const eventsDiv = document.getElementById('events');
      eventsDiv.innerHTML='';
      if(evs.length===0){eventsDiv.textContent='No events found';}
      evs.forEach(ev=>{
        const div=document.createElement('div');
        div.className='event';
        const ts = ev.lastTimestamp ? new Date(ev.lastTimestamp*1000).toLocaleString()+': ' : '';
        div.textContent=ts+'['+ev.type+'] '+ev.reason+' - '+ev.message;
        eventsDiv.appendChild(div);
      });
    }catch(e){console.error(e);}
  }catch(e){console.error(e);}
}

async function loadLogs(p){
  try{
    const params='namespace='+encodeURIComponent(p.namespace)+'&khcheck='+encodeURIComponent(currentCheck)+'&pod='+encodeURIComponent(p.name);
    const res = await (await fetch('/api/logs?'+params)).json();
    document.getElementById('pod-info').textContent='Started: '+(res.startTime?new Date(res.startTime*1000).toLocaleString():'')+' Duration: '+res.durationSeconds+'s Phase: '+res.phase;
    const logElem=document.getElementById('logs');
    logElem.textContent=res.logs || '';
    if (p.phase === 'Running'){
      streamLogs('/api/logs/stream?'+params, logElem);
    }
  }catch(e){console.error(e);}
}

async function streamLogs(url, elem){
  try{
    const resp = await fetch(url);
    if(!resp.body) return;
    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    while(true){
      const {value, done} = await reader.read();
      if(done) break;
      elem.textContent += decoder.decode(value);
    }
  }catch(e){console.error(e);}
}

setInterval(refresh,5000);
window.onload = ()=>{initTheme(); refresh();};
</script>
</head>
<body class="d-flex flex-column min-vh-100">
  <header class="d-flex align-items-center justify-content-between bg-primary text-white p-3">
    <h1 class="h4 m-0">Kuberhealthy Status</h1>
    <button id="themeToggle" class="btn btn-sm btn-light" onclick="setTheme(document.documentElement.getAttribute('data-bs-theme')==='dark'?'light':'dark')">☀️</button>
  </header>
<div id="main">
  <div id="menu" class="bg-body-tertiary"></div>
  <div id="content" class="bg-body"><h2>Select a check</h2></div>
</div>
<footer class="bg-body-secondary text-center p-2">Powered by Kuberhealthy</footer>
</body>
</html>
`
const defaultRunInterval = time.Minute * 10

// requestLogger logs incoming requests before they reach a handler.
func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// determine the source IP making the request
		ip := r.Header.Get("X-Forwarded-For")
		if ip != "" {
			parts := strings.Split(ip, ",")
			ip = strings.TrimSpace(parts[0])
		} else {
			// ignore the port on the remote address
			ip, _, _ = net.SplitHostPort(r.RemoteAddr)
		}

		// capture the user agent string
		ua := r.UserAgent()

		// log the IP, user agent, HTTP method, and path
		log.WithFields(log.Fields{
			"ip":     ip,
			"ua":     ua,
			"method": r.Method,
			"path":   r.URL.Path,
		}).Info("Client request")

		// continue handling the request
		next.ServeHTTP(w, r)
	})
}

// newServeMux configures and returns a mux with all web handlers mounted.
func newServeMux() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("/metrics", func(w http.ResponseWriter, r *http.Request) {
		if err := prometheusMetricsHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if err := healthCheckHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/json", func(w http.ResponseWriter, r *http.Request) {
		if err := healthCheckHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/api/pods", func(w http.ResponseWriter, r *http.Request) {
		if err := podListHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		if err := eventListHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/api/logs", func(w http.ResponseWriter, r *http.Request) {
		if err := podLogsHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/api/logs/stream", func(w http.ResponseWriter, r *http.Request) {
		if err := podLogsStreamHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/openapi.yaml", func(w http.ResponseWriter, r *http.Request) {
		if err := openapiYAMLHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/openapi.json", func(w http.ResponseWriter, r *http.Request) {
		if err := openapiJSONHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	mux.HandleFunc("/check", func(w http.ResponseWriter, r *http.Request) {
		if err := checkReportHandler(w, r); err != nil {
			log.Errorln("checkStatus endpoint error:", err)
		}
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			if err := checkReportHandler(w, r); err != nil {
				log.Errorln("checkStatus endpoint error:", err)
			}
			return
		}
		if err := statusPageHandler(w, r); err != nil {
			log.Errorln(err)
		}
	})

	return mux
}

// StartWebServer starts the web server with all handlers attached to the muxer.
func StartWebServer() error {
	mux := newServeMux()
	log.Infoln("Starting web services on port", GlobalConfig.ListenAddress)
	return http.ListenAndServe(GlobalConfig.ListenAddress, requestLogger(mux))
}

// statusPageHandler serves a basic HTML page that polls the JSON endpoint
// to show the status of all configured checks.
func statusPageHandler(w http.ResponseWriter, r *http.Request) error {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, err := io.WriteString(w, statusPageHTML)
	if err != nil {
		log.Warningln("Error writing status page:", err)
	}
	return err
}

// renderOpenAPISpec writes the OpenAPI spec as JSON.
func renderOpenAPISpec(w http.ResponseWriter) error {
	data, err := os.ReadFile("./openapi.yaml")
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}

	jsonData, err := yaml.YAMLToJSON(data)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_, err = w.Write(jsonData)
	return err
}

// openapiYAMLHandler serves the OpenAPI spec at /openapi.yaml as JSON.
func openapiYAMLHandler(w http.ResponseWriter, r *http.Request) error {
	return renderOpenAPISpec(w)
}

// openapiJSONHandler serves the OpenAPI spec at /openapi.json as JSON.
func openapiJSONHandler(w http.ResponseWriter, r *http.Request) error {
	return renderOpenAPISpec(w)
}

type podSummary struct {
	Name            string            `json:"name"`
	Namespace       string            `json:"namespace"`
	Phase           string            `json:"phase"`
	StartTime       int64             `json:"startTime,omitempty"`
	DurationSeconds int64             `json:"durationSeconds,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Annotations     map[string]string `json:"annotations,omitempty"`
}

type podLogResponse struct {
	podSummary
	Logs string `json:"logs"`
}

type eventSummary struct {
	Message       string `json:"message"`
	Reason        string `json:"reason"`
	Type          string `json:"type"`
	LastTimestamp int64  `json:"lastTimestamp,omitempty"`
}

// podListHandler returns a list of pods for a given check.
func podListHandler(w http.ResponseWriter, r *http.Request) error {
	khcheck := r.URL.Query().Get("khcheck")
	namespace := r.URL.Query().Get("namespace")
	if khcheck == "" || namespace == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if KHController == nil || KHController.Client == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	khc := &kuberhealthycheckv2.KuberhealthyCheck{}
	if err := KHController.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: khcheck}, khc); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return err
	}
	pods := &v1.PodList{}
	if err := KHController.Client.List(r.Context(), pods, client.InNamespace(namespace), client.MatchingLabels{"khcheck": khcheck}); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	summaries := make([]podSummary, 0, len(pods.Items))
	for _, p := range pods.Items {
		s := podSummary{Name: p.Name, Namespace: p.Namespace, Phase: string(p.Status.Phase), Labels: p.Labels, Annotations: p.Annotations}
		if p.Status.StartTime != nil {
			s.StartTime = p.Status.StartTime.Unix()
			s.DurationSeconds = int64(podRunDuration(&p).Seconds())
		}
		summaries = append(summaries, s)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(summaries)
}

// eventListHandler returns events for a given check.
func eventListHandler(w http.ResponseWriter, r *http.Request) error {
	khcheck := r.URL.Query().Get("khcheck")
	namespace := r.URL.Query().Get("namespace")
	if khcheck == "" || namespace == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	fs := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=KuberhealthyCheck", khcheck)
	evList, err := kubeClient.CoreV1().Events(namespace).List(r.Context(), metav1.ListOptions{FieldSelector: fs})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	summaries := make([]eventSummary, 0, len(evList.Items))
	for _, e := range evList.Items {
		s := eventSummary{Message: e.Message, Reason: e.Reason, Type: e.Type}
		if !e.LastTimestamp.IsZero() {
			s.LastTimestamp = e.LastTimestamp.Unix()
		}
		summaries = append(summaries, s)
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(summaries)
}

// podLogsHandler returns logs and details for a specific pod.
func podLogsHandler(w http.ResponseWriter, r *http.Request) error {
	podName := r.URL.Query().Get("pod")
	namespace := r.URL.Query().Get("namespace")
	khcheck := r.URL.Query().Get("khcheck")
	if podName == "" || namespace == "" || khcheck == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if KHController == nil || KHController.Client == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	pod := &v1.Pod{}
	if err := KHController.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: podName}, pod); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if pod.Labels["khcheck"] != khcheck {
		w.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("pod not part of khcheck")
	}
	khc := &kuberhealthycheckv2.KuberhealthyCheck{}
	if err := KHController.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: khcheck}, khc); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return err
	}
	if kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	req := kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
	stream, err := req.Stream(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	defer stream.Close()
	b, err := io.ReadAll(stream)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	resp := podLogResponse{
		podSummary: podSummary{
			Name:        podName,
			Namespace:   namespace,
			Phase:       string(pod.Status.Phase),
			Labels:      pod.Labels,
			Annotations: pod.Annotations,
		},
		Logs: string(b),
	}
	if pod.Status.StartTime != nil {
		resp.StartTime = pod.Status.StartTime.Unix()
		resp.DurationSeconds = int64(podRunDuration(pod).Seconds())
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	return json.NewEncoder(w).Encode(resp)
}

// podLogsStreamHandler streams logs for a specific pod in real time.
func podLogsStreamHandler(w http.ResponseWriter, r *http.Request) error {
	podName := r.URL.Query().Get("pod")
	namespace := r.URL.Query().Get("namespace")
	khcheck := r.URL.Query().Get("khcheck")
	if podName == "" || namespace == "" || khcheck == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if KHController == nil || KHController.Client == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	pod := &v1.Pod{}
	if err := KHController.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: podName}, pod); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if pod.Labels["khcheck"] != khcheck {
		w.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("pod not part of khcheck")
	}
	if pod.Status.Phase != v1.PodRunning {
		w.WriteHeader(http.StatusConflict)
		return fmt.Errorf("pod not running")
	}
	khc := &kuberhealthycheckv2.KuberhealthyCheck{}
	if err := KHController.Client.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: khcheck}, khc); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return err
	}
	if kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	req := kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{Follow: true})
	stream, err := req.Stream(r.Context())
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	defer stream.Close()
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	flusher, ok := w.(http.Flusher)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("streaming not supported")
	}
	buf := make([]byte, 1024)
	for {
		n, err := stream.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return werr
			}
			flusher.Flush()
		}
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}
	}
}

// podRunDuration returns the runtime of a pod.
func podRunDuration(p *v1.Pod) time.Duration {
	if p.Status.StartTime == nil {
		return 0
	}
	start := p.Status.StartTime.Time
	if p.Status.Phase == v1.PodRunning {
		return time.Since(start)
	}
	for _, cs := range p.Status.ContainerStatuses {
		if cs.State.Terminated != nil {
			return cs.State.Terminated.FinishedAt.Sub(cs.State.Terminated.StartedAt.Time)
		}
	}
	return time.Since(start)
}

// PodReportInfo holds info about an incoming IP to the external check reporting endpoint
type PodReportInfo struct {
	Name      string
	UUID      string
	Namespace string
}

// function variables allow tests to stub dependencies
var (
	validateUsingRequestHeaderFunc  = validateUsingRequestHeader
	validatePodReportBySourceIPFunc = validatePodReportBySourceIP
	storeCheckStateFunc             = storeCheckState
)

// validateExternalRequest calls the Kubernetes API to fetch details about a pod using a selector string.
// It validates that the pod is allowed to report the status of a check. The pod is also expected
// to have the environment variable KH_CHECK_NAME
func validateExternalRequest(ctx context.Context, selector string) (PodReportInfo, error) {

	var podUUID string
	var podCheckName string
	var podCheckNamespace string

	reportInfo := PodReportInfo{}

	// fetch the pod from the api using a specified selector. We keep retrying for some time to avoid kubernetes control
	// plane api race conditions wherein fast reporting pods are not found in pod listings
	pod, err := fetchPodBySelectorForDuration(ctx, selector, time.Minute)
	if err != nil {
		return reportInfo, err
	}

	// set the pod namespace and name from the returned metadata
	podCheckName = pod.Annotations[envs.KHCheckNameAnnotationKey]
	if len(podCheckName) == 0 {
		return reportInfo, errors.New("error finding check name annotation on calling pod with selector: " + selector)
	}

	podCheckNamespace = pod.GetNamespace()
	log.Debugln("Found check named", podCheckName, "in namespace", podCheckNamespace)

	// pile up all the env vars for searching
	var envVars []v1.EnvVar
	// we found our pod, lets return all its env vars from all its containers
	for _, container := range pod.Spec.Containers {
		envVars = append(envVars, container.Env...)
	}

	log.Debugln("Env vars found on pod with selector", selector, envVars)

	// validate that the environment variables we expect are in place based on the check's name and UUID
	// compared to what is in the khcheck custom resource
	var foundUUID bool
	for _, e := range envVars {
		log.Debugln("Checking environment variable on calling pod:", e.Name, e.Value)
		if e.Name == envs.KHRunUUID {
			log.Debugln("Found value on calling pod", selector, "value:", envs.KHRunUUID, e.Value)
			podUUID = e.Value
			foundUUID = true
		}
	}

	// verify that we found the UUID
	if !foundUUID {
		return reportInfo, errors.New("error finding environment variable on remote pod: " + envs.KHRunUUID)
	}

	// we know that we have a UUID and check name now, so lets check their validity.  First, we sanity check.
	if len(podCheckName) < 1 {
		return reportInfo, errors.New("pod check name was invalid or unset")
	}
	if len(podCheckNamespace) < 1 {
		return reportInfo, errors.New("pod check namespace was invalid or unset")
	}
	if len(podUUID) < 1 {
		return reportInfo, errors.New("pod uuid was invalid or unset")
	}

	// create a report to send back to the function invoker
	reportInfo.Name = podCheckName
	reportInfo.Namespace = podCheckNamespace
	reportInfo.UUID = podUUID

	// next, we check the uuid against the check name to see if this uuid is the expected one.  if it isn't,
	// we return an error
	whitelisted, err := isUUIDWhitelistedForCheck(podCheckName, podCheckNamespace, podUUID)
	if err != nil {
		return reportInfo, fmt.Errorf("failed to fetch whitelisted UUID for check with error: %w", err)
	}
	if !whitelisted {
		return reportInfo, errors.New("pod was not properly whitelisted for reporting status of check " + podCheckName + " with uuid " + podUUID + " and namespace " + podCheckNamespace)
	}

	return reportInfo, nil
}

// prometheusMetricsHandler is a handler for all prometheus metrics requests
func prometheusMetricsHandler(w http.ResponseWriter, r *http.Request) error {
	// log.Infoln("Client connected to prometheus metrics endpoint from", r.RemoteAddr, r.UserAgent())
	state := getCurrentState([]string{})

	m := metrics.GenerateMetrics(state, GlobalConfig.PromMetricsConfig)
	// write summarized health check results back to caller
	_, err := w.Write([]byte(m))
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// healthCheckHandler returns the current status of checks loaded into Kuberhealthy
// as JSON to the client. Respects namespace requests via URL query parameters (i.e. /?namespace=default)
func healthCheckHandler(w http.ResponseWriter, r *http.Request) error {
	// log.Infoln("Client connected to status page from", r.RemoteAddr, r.UserAgent())

	// If a request body was supplied, throw an error to ensure that checks don't report into the wrong url
	body, err := io.ReadAll(r.Body)
	if len(body) > 0 {
		log.Warningln("Unexpected body from status page request. Verify check is reporting to the right status url", r.RemoteAddr, r.RequestURI)
		w.WriteHeader(http.StatusBadRequest)
		return err
	}

	// get URL query parameters if there are any
	values := r.URL.Query()
	namespaceValue := values.Get("namespace")
	// .Get() will return an "" if there is no value associated -- we do not want to pass "" as a requested namespace
	var namespaces []string
	if len(namespaceValue) != 0 {
		namespaceSplits := strings.Split(namespaceValue, ",")
		// a query like (/?namespace=,) will cause .Split() to return an array of two empty strings ["", ""]
		// so we need to filter those out
		for _, namespaceSplit := range namespaceSplits {
			if len(namespaceSplit) != 0 {
				namespaces = append(namespaces, namespaceSplit)
			}
		}
	}

	// fetch the current status from our khcheck resources
	state := getCurrentState(namespaces)

	// write summarized health check results back to caller
	err = state.WriteHTTPStatusResponse(w)
	if err != nil {
		log.Warningln("Error writing health check results to caller:", err)
	}
	return err
}

// checkReportHandler handles requests coming from external checkers reporting their status.
// This endpoint checks that the external check report is coming from the correct UUID or pod IP before recording
// the reported status of the corresponding external check.  This endpoint expects a JSON payload of
// the `State` struct found in the github.com/kuberhealthy/kuberhealthy/v2/pkg/health package.  The request
// causes a check of the calling pod's spec via the API to ensure that the calling pod is expected
// to be reporting its status.
func checkReportHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		w.WriteHeader(http.StatusUnsupportedMediaType)
		return nil
	}

	// make a request ID for tracking this request
	requestID := "web: " + uuid.New().String()

	ctx := r.Context()

	// log.Println("webserver:", requestID, "Client connected to check report handler from", r.UserAgent())

	// Validate request using the kh-run-uuid header. If the header doesn't exist, or there's an error with validation,
	// validate using the pod's remote IP.
	log.Println("webserver:", requestID, "validating external check status report from its reporting kuberhealthy run uuid:", r.Header.Get("kh-run-uuid"))
	podReport, reportValidated, err := validateUsingRequestHeaderFunc(ctx, r)
	if err != nil {
		log.Println("webserver:", requestID, "Failed to look up pod by its kh-run-uuid header:", r.Header.Get("kh-run-uuid"), err)
	}

	// If the check uuid header is missing, attempt to validate using calling pod's source IP
	if !reportValidated {
		log.Println("webserver:", requestID, "validating external check status report from the pod's remote IP:", r.RemoteAddr)
		podReport, err = validatePodReportBySourceIPFunc(ctx, r)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("webserver:", requestID, "Failed to look up pod by its IP:", r.RemoteAddr, err)
			return nil
		}
	}
	log.Println("webserver:", requestID, "Calling pod is", podReport.Name, "in namespace", podReport.Namespace)

	// append pod info to request id for easy check tracing in logs
	requestID = requestID + " (" + podReport.Namespace + "/" + podReport.Name + ")"

	// fetch the khcheck for event recording when controller is available
	var khCheck *kuberhealthycheckv2.KuberhealthyCheck
	if KHController != nil && KHController.Client != nil {
		nn := types.NamespacedName{Name: podReport.Name, Namespace: podReport.Namespace}
		khCheck = &kuberhealthycheckv2.KuberhealthyCheck{}
		if err := KHController.Client.Get(ctx, nn, khCheck); err != nil {
			// if we cannot fetch, skip event recording
			log.Println("webserver:", requestID, "failed to fetch khcheck for event recording:", err)
			khCheck = nil
		}
	}

	// ensure the client is sending a valid payload in the request body
	b, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Failed to read request body:", err.Error(), r.RemoteAddr)
		return nil
	}
	log.Debugln("Check report body:", string(b))

	// decode the bytes into a status struct as used by the client
	state := health.Report{}
	err = json.Unmarshal(b, &state)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Failed to unmarshal state json:", err, r.RemoteAddr)
		return nil
	}
	log.Debugf("Check report after unmarshal: +%v\n", state)

	// ensure that reports do not contain errors when OK is true
	if state.OK && len(state.Errors) > 0 {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Client attempted to report OK true with error strings")
		return nil
	}

	// ensure that if ok is set to false, then an error is provided
	if !state.OK {
		if len(state.Errors) == 0 {
			w.WriteHeader(http.StatusBadRequest)
			log.Println("webserver:", requestID, "Client attempted to report OK false without any error strings")
			return nil
		}
		for _, e := range state.Errors {
			if len(e) == 0 {
				w.WriteHeader(http.StatusBadRequest)
				log.Println("webserver:", requestID, "Client attempted to report a blank error string")
				return nil
			}
		}
	}

	// checkRunDuration := time.Duration(0).String()

	// checkDetails := stateReflector.CurrentStatus().CheckDetails
	// checkRunDuration = checkDetails[podReport.Namespace+"/"+podReport.Name].RunDuration

	// create a details object from our incoming status report before storing it on the khcheck status field
	details := &kuberhealthycheckv2.KuberhealthyCheckStatus{}
	details.Errors = state.Errors
	details.OK = state.OK
	// details.RunDuration = checkRunDuration
	details.Namespace = podReport.Namespace
	details.CurrentUUID = podReport.UUID

	// since the check is validated, we can proceed to update the status now
	log.Println("webserver:", requestID, "Setting check with name", podReport.Name, "in namespace", podReport.Namespace, "to 'OK' state:", details.OK, "uuid", details.CurrentUUID)
	err = storeCheckStateFunc(podReport.Name, podReport.Namespace, details)
	if err != nil {
		if khCheck != nil && KHController != nil && KHController.Kuberhealthy != nil && KHController.Kuberhealthy.Recorder != nil {
			KHController.Kuberhealthy.Recorder.Eventf(khCheck, v1.EventTypeWarning, "CheckReportFailed", "failed to store check state: %v", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("webserver:", requestID, "failed to store check state for %s: %w", podReport.Name, err)
		return fmt.Errorf("failed to store check state for %s: %w", podReport.Name, err)
	}

	if khCheck != nil && KHController != nil && KHController.Kuberhealthy != nil && KHController.Kuberhealthy.Recorder != nil {
		if state.OK {
			KHController.Kuberhealthy.Recorder.Event(khCheck, v1.EventTypeNormal, "CheckReported", "check reported OK")
		} else {
			KHController.Kuberhealthy.Recorder.Eventf(khCheck, v1.EventTypeWarning, "CheckReported", strings.Join(state.Errors, "; "))
		}
	}

	// write ok back to caller
	w.WriteHeader(http.StatusOK)
	log.Println("webserver:", requestID, "Request completed successfully.")
	return nil
}

// fetchPodBySelectorForDuration attempts to fetch a pod by a specified selector repeatedly for the supplied duration.
// If the pod is found, then we return it.  If the pod is not found after the duration, we return an error
func fetchPodBySelectorForDuration(ctx context.Context, selector string, d time.Duration) (v1.Pod, error) {
	endTime := time.Now().Add(d)

	for {
		if time.Now().After(endTime) {
			return v1.Pod{}, errors.New("Failed to fetch source pod with selector " + selector + " after trying for " + d.String())
		}

		p, err := fetchPodBySelector(ctx, selector)
		if err != nil {
			log.Warningln("was unable to find calling pod with selector " + selector + " while watching for duration. Error: " + err.Error())
			time.Sleep(time.Second)
			continue
		}

		return p, err
	}
}

// getCheckStatus fetches the status section of a kuberhealthy check resource and returns it
func getCheckStatus(checkName string, checkNamespace string) (*kuberhealthycheckv2.KuberhealthyCheckStatus, error) {

	// TODO
	return &kuberhealthycheckv2.KuberhealthyCheckStatus{}, nil
}

// isUUIDWhitelistedForCheck determines if the supplied uuid is whitelisted for the
// check with the supplied name.  Only one UUID can be whitelisted at a time.
// Operations are not atomic.  Whitelisting prevents expired or invalidated pods from
// reporting into the status endpoint when they shouldn't be.
func isUUIDWhitelistedForCheck(checkName string, checkNamespace string, uuid string) (bool, error) {

	// get the item in question
	checkStatus, err := getCheckStatus(checkName, checkNamespace)
	if err != nil {
		return false, err
	}

	log.Debugln("Validating current UUID", checkStatus.CurrentUUID, "vs incoming UUID:", uuid)
	if checkStatus.CurrentUUID == uuid {
		return true, nil
	}
	return false, nil
}

// getCurrentState fetches the current state of all checks from requested namespaces
// their CRD objects and returns the summary as a health.State. Without a requested namespace,
// this will return the state of ALL found checks.
// Failures to fetch CRD state return an error.
func getCurrentState(namespaces []string) health.State {
	return getCurrentStatusForNamespaces(namespaces)
}

// getCurrentState fetches the current state of all checks from the requested namespaces
// their CRD objects and returns the summary as a health.State.
// Failures to fetch CRD state return an error.
func getCurrentStatusForNamespaces(namespaces []string) health.State {
	state := health.NewState()
	if KHController == nil {
		state.OK = false
		state.AddError("kuberhealthy controller not initialized")
		return state
	}

	ctx := context.Background()
	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(kuberhealthycheckv2.GroupVersion.WithKind("KuberhealthyCheckList"))
	var opts []client.ListOption
	if len(namespaces) == 1 {
		opts = append(opts, client.InNamespace(namespaces[0]))
	}
	if err := KHController.Client.List(ctx, uList, opts...); err != nil {
		state.OK = false
		state.AddError(err.Error())
		return state
	}

	for _, u := range uList.Items {
		if len(namespaces) > 1 && !containsString(u.GetNamespace(), namespaces) {
			continue
		}

		runInterval := runIntervalForCheck(&u)

		var check kuberhealthycheckv2.KuberhealthyCheck
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &check); err != nil {
			log.Errorf("failed to convert check %s/%s: %v", u.GetNamespace(), u.GetName(), err)
			continue
		}

		status := health.CheckDetail{KuberhealthyCheckStatus: check.Status}
		if status.Namespace == "" {
			status.Namespace = check.Namespace
		}
		if check.Status.LastRunUnix != 0 {
			next := time.Unix(check.Status.LastRunUnix, 0).Add(runInterval)
			status.NextRunUnix = next.Unix()
		}
		state.CheckDetails[check.Name] = status
		if !status.OK {
			state.OK = false
			state.AddError(status.Errors...)
		}
	}

	return state
}

func runIntervalForCheck(u *unstructured.Unstructured) time.Duration {
	if v, found, err := unstructured.NestedString(u.Object, "spec", "runInterval"); err == nil && found && v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
		log.Errorf("invalid runInterval %q on check %s/%s", v, u.GetNamespace(), u.GetName())
	}
	return defaultRunInterval
}

// validateUsingRequestHeader gets the header `kh-run-uuid` value from the request and forms a selector with it to
// validate that the request is coming from a kuberhealthy check pod
func validateUsingRequestHeader(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {

	var podReport PodReportInfo
	var err error
	if len(r.Header.Get("kh-run-uuid")) == 0 {
		return podReport, false, nil
	}
	selector := "kuberhealthy-run-id=" + r.Header.Get("kh-run-uuid")
	podReport, err = validateExternalRequest(ctx, selector)
	if err != nil {
		return podReport, false, err
	}
	return podReport, true, nil
}

// validatePodReportBySourceIP parses the remoteAddr from the request and forms a selector with the remote IP to
// validate that the request is coming from a kuberhealthy check pod
func validatePodReportBySourceIP(ctx context.Context, r *http.Request) (PodReportInfo, error) {

	var podReport PodReportInfo
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return podReport, err
	}
	selector := "status.podIP==" + ip + ",status.phase==Running"
	podReport, err = validateExternalRequest(ctx, selector)
	if err != nil {
		return podReport, err
	}
	return podReport, nil
}

// fetchPodBySelector fetches the pod by it's `kuberhealthy-run-id` label selector or by its `status.podIP` field selector
func fetchPodBySelector(ctx context.Context, selector string) (v1.Pod, error) {
	// var pod v1.Pod

	// podClient := kubernetesClient.CoreV1().Pods(TargetNamespace)

	// // Use either label selector or field selector depending on the selector string passed through
	// // LabelSelector: "kuberhealthy-run-id=" + uuid,
	// // FieldSelector: "status.podIP==" + remoteIP + ",status.phase==Running",
	// var listOptions metav1.ListOptions
	// if strings.Contains(selector, "kuberhealthy-run-id") {
	// 	listOptions = metav1.ListOptions{
	// 		LabelSelector: selector,
	// 	}
	// }

	// if strings.Contains(selector, "status.podIP") {
	// 	listOptions = metav1.ListOptions{
	// 		FieldSelector: selector,
	// 	}
	// }

	// podList, err := podClient.List(ctx, listOptions)
	// if err != nil {
	// 	return pod, errors.New("failed to fetch pod with selector " + selector + " with error: " + err.Error())
	// }

	// // ensure that we only got back one pod, because two means something awful has happened and 0 means we
	// // didnt find one
	// if len(podList.Items) == 0 {
	// 	return pod, errors.New("failed to find a pod with selector " + selector)
	// }
	// if len(podList.Items) > 1 {
	// 	return pod, errors.New("failed to fetch pod with selector " + selector + " - found two or more with same label")
	// }

	// // check if the pod has containers
	// if len(podList.Items[0].Spec.Containers) == 0 {
	// 	return pod, errors.New("failed to fetch environment variables from pod with selector" + selector + " - pod had no containers")
	// }

	// return podList.Items[0], nil

	return v1.Pod{}, nil
}

// storeCheckState stores the check status on its cluster CRD
func storeCheckState(checkName string, checkNamespace string, khcheck *kuberhealthycheckv2.KuberhealthyCheckStatus) error {
	// ensure the CRD resource exists
	if err := ensureCheckResourceExists(checkName, checkNamespace); err != nil {
		return err
	}

	// put the status on the CRD from the check
	err := setCheckStatus(checkName, checkNamespace, khcheck)

	//TODO: Make this retry of updating custom resources repeatable
	//
	// We commonly see a race here with the following type of error:
	// "Error storing CRD state for check: pod-restarts in namespace kuberhealthy Operation cannot be fulfilled on kuberhealthychecks, comcast.github.io \"pod-restarts\": the object
	// has been modified; please apply your changes to the latest version and try again"
	//
	// If we see this error, we fetch the updated object, re-apply our changes, and try again
	delay := time.Duration(time.Second * 1)
	maxTries := 7
	tries := 0
	for err != nil && strings.Contains(err.Error(), "the object has been modified") {
		// if too many retries have occurred, we fail up the stack further
		if tries > maxTries {
			return fmt.Errorf("failed to update khcheck status for check %s in namespace %s after %d with error %w", checkName, checkNamespace, maxTries, err)
		}
		log.Warnln("Failed to update khcheck status because object was modified by another process.  Retrying in " + delay.String() + ".  Try " + strconv.Itoa(tries) + " of " + strconv.Itoa(maxTries) + ".")

		// sleep and double the delay between checks (exponential backoff)
		time.Sleep(delay)
		delay = delay + delay

		// try setting the check state again
		err = setCheckStatus(checkName, checkNamespace, khcheck)

		// count how many times we've retried
		tries++
	}

	return err
}

// ensureCheckResourceExists ensures that the khcheck resource exists so that status can be updated
func ensureCheckResourceExists(checkName string, checkNamespace string) error {
	if KHController == nil {
		return fmt.Errorf("kuberhealthy controller not initialized")
	}

	ctx := context.Background()
	nn := types.NamespacedName{Name: checkName, Namespace: checkNamespace}
	khCheck := &kuberhealthycheckv2.KuberhealthyCheck{}
	if err := KHController.Client.Get(ctx, nn, khCheck); err != nil {
		if apierrors.IsNotFound(err) {
			khCheck.ObjectMeta = metav1.ObjectMeta{Name: checkName, Namespace: checkNamespace}
			if err := KHController.Client.Create(ctx, khCheck); err != nil {
				return fmt.Errorf("failed to create khcheck %s/%s: %w", checkNamespace, checkName, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get khcheck %s/%s: %w", checkNamespace, checkName, err)
	}
	return nil
}

// setCheckStatus sets the status on the khcheck custom resource
func setCheckStatus(checkName string, checkNamespace string, khcheck *kuberhealthycheckv2.KuberhealthyCheckStatus) error {
	if KHController == nil {
		return fmt.Errorf("kuberhealthy controller not initialized")
	}

	ctx := context.Background()
	nn := types.NamespacedName{Name: checkName, Namespace: checkNamespace}
	khCheck := &kuberhealthycheckv2.KuberhealthyCheck{}
	if err := KHController.Client.Get(ctx, nn, khCheck); err != nil {
		return fmt.Errorf("failed to get khcheck: %w", err)
	}

	khCheck.Status = *khcheck
	if khCheck.Status.Namespace == "" {
		khCheck.Status.Namespace = checkNamespace
	}

	if err := KHController.Client.Status().Update(ctx, khCheck); err != nil {
		return fmt.Errorf("failed to update khcheck status: %w", err)
	}

	return nil
}
