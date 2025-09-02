package main

import (
	"context"
	"encoding/json"
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
	"github.com/kuberhealthy/kuberhealthy/v3/internal/health"
	"github.com/kuberhealthy/kuberhealthy/v3/internal/metrics"
	khapi "github.com/kuberhealthy/kuberhealthy/v3/pkg/api"
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
<script src="https://cdn.tailwindcss.com"></script>
<script>
let checks = {};
let currentCheck = '';
let nextRunTimer = null;

function setTheme(t){
  if(t==='dark'){ document.documentElement.classList.add('dark'); }
  else{ document.documentElement.classList.remove('dark'); }
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

function updateNextRun(unix){
  const elem=document.getElementById('nextRun');
  if(!elem){return;}
  elem.textContent=formatDuration(unix*1000-Date.now());
}

async function refresh(){
  try{
    const resp = await fetch('/json');
    const data = await resp.json();
    checks = data.CheckDetails || {};
    const menu = document.getElementById('menu');
    menu.innerHTML = '';
    const heading=document.createElement('div');
    heading.textContent='Checks';
    heading.className='p-3 font-semibold text-gray-700 dark:text-gray-300';
    menu.appendChild(heading);
    const now = Date.now();
    const names = Object.keys(checks);
    names.forEach(function(name){
      const st = checks[name];
      let icon = st.ok ? '✅' : '❌';
      if (st.podName){ icon = '⏳'; }
      const div = document.createElement('div');
      div.className = 'p-2 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-900 dark:text-gray-100';
      let label = icon + ' ' + name;
      if (st.nextRunUnix){
        const diff = st.nextRunUnix*1000 - now;
        label += ' (' + formatDuration(diff) + ')';
      }
      div.textContent = label;
      div.onclick = ()=>showCheck(name);
      menu.appendChild(div);
    });
    if(currentCheck===''){
      const content=document.getElementById('content');
      content.innerHTML='';
      const grid=document.createElement('div');
      grid.className='grid gap-4 md:grid-cols-2';
      names.forEach(function(name){
        const st=checks[name];
        const state=st.podName?'running':(st.ok?'ok':'error');
        const status=st.podName?'Running':(st.ok?'OK':'Fail');
        const err=st.errors&&st.errors.length?'<p><span class="font-semibold">Error:</span> '+st.errors[0]+'</p>':'';
        const card=document.createElement('div');
        card.className='p-4 bg-white dark:bg-gray-900 rounded shadow cursor-pointer';
        card.innerHTML=
          '<h3 class="text-xl font-semibold mb-2">'+name+'</h3>'+
          '<p><span class="font-semibold">Status:</span> '+status+'</p>'+
          err+
          '<p><span class="font-semibold">State:</span> '+state+'</p>'+
          '<p><span class="font-semibold">Namespace:</span> '+st.namespace+'</p>';
        card.onclick=()=>showCheck(name);
        grid.appendChild(card);
      });
      content.appendChild(grid);
    }
  }catch(e){ console.error('failed to fetch status', e); }
}

async function showCheck(name){
  if(nextRunTimer){ clearInterval(nextRunTimer); }
  currentCheck = name;
  const st = checks[name];
  if(!st){return;}
  const content = document.getElementById('content');
  content.innerHTML=
    '<h2 class="text-2xl font-bold mb-4">'+name+'</h2>'+
    '<div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4">'+
      '<h3 class="text-xl font-semibold mb-2">Overview</h3>'+
      '<p class="mb-2"><span class="font-semibold">Status:</span> <span class="'+(st.podName?'text-blue-600':(st.ok?'text-green-600':'text-red-600'))+'">'+(st.podName?'Running':(st.ok?'OK':'Fail'))+'</span></p>'+
      '<p class="mb-2"><span class="font-semibold">Namespace:</span> '+st.namespace+'</p>'+
      (st.podName?'<p class="mb-2"><span class="font-semibold">Pod:</span> '+st.podName+'</p>':(st.nextRunUnix?'<p class="mb-2 flex items-center gap-2"><span class="font-semibold">Next run in:</span> <span id="nextRun"></span><button id="runNow" class="px-2 py-1 text-xs bg-blue-600 text-white rounded">Run now</button></p>':''))+
      (st.lastRunUnix?'<p class="mb-2"><span class="font-semibold">Last run:</span> '+new Date(st.lastRunUnix*1000).toLocaleString()+'</p>':'')+
      (st.errors && st.errors.length ? '<div class="mb-2"><span class="font-semibold text-red-600">Errors:</span><ul class="list-disc list-inside">'+st.errors.map(e=>'<li>'+e+'</li>').join('')+'</ul></div>' : '')+
    '</div>'+
    '<div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4"><h3 class="text-xl font-semibold mb-2">Events</h3><div id="events" class="text-gray-900 dark:text-gray-100">loading...</div></div>'+
    '<details class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4"><summary class="text-xl font-semibold cursor-pointer mb-2">Prior Pod Runs</summary><div id="pods" class="text-gray-900 dark:text-gray-100 mt-2"></div></details>'+
    '<div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4"><h3 class="text-xl font-semibold mb-2">Pod Details</h3><div id="pod-info" class="text-gray-900 dark:text-gray-100"></div></div>'+
    '<div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4"><h3 class="text-xl font-semibold mb-2">Logs</h3><pre id="logs" class="whitespace-pre-wrap bg-gray-100 dark:bg-gray-800 p-4 text-gray-900 dark:text-gray-100 rounded shadow-inner"></pre></div>';
  if(st.nextRunUnix && !st.podName){
    updateNextRun(st.nextRunUnix);
    nextRunTimer=setInterval(()=>updateNextRun(st.nextRunUnix),1000);
    const btn=document.getElementById('runNow');
    if(btn){
      btn.onclick=async ()=>{
        btn.disabled=true;
        try{
          await fetch('/api/run?namespace='+encodeURIComponent(st.namespace)+'&khcheck='+encodeURIComponent(name),{method:'POST'});
          await refresh();
          await showCheck(name);
        }catch(e){console.error(e);}
        btn.disabled=false;
      };
    }
  }
  try{
    const pods = await (await fetch('/api/pods?namespace='+encodeURIComponent(st.namespace)+'&khcheck='+encodeURIComponent(name))).json();
    pods.sort((a,b)=>(b.startTime||0)-(a.startTime||0));
    const podsDiv=document.getElementById('pods');
    podsDiv.innerHTML='';
    let mainPod;
    if(st.podName){
      mainPod = pods.find(p=>p.name===st.podName);
      if(mainPod){ loadLogs(mainPod); } else { loadLogs({name: st.podName, namespace: st.namespace, phase:'Running'}); }
    }else if(pods.length){
      mainPod = pods[0];
      loadLogs(mainPod);
    }
    const priorPods=pods.filter(p=>!mainPod || p.name!==mainPod.name);
    if(priorPods.length===0){
      podsDiv.parentElement.remove();
    }
    priorPods.forEach(p=>{
      const div=document.createElement('div');
      div.className='cursor-pointer p-1 hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-900 dark:text-gray-100';
      const start=p.startTime?new Date(p.startTime*1000).toLocaleString():'';
      const end=p.startTime?new Date((p.startTime+p.durationSeconds)*1000).toLocaleString():'';
      div.textContent=p.name+' - '+start+' → '+end+' ('+p.phase+')';
      div.onclick=()=>loadLogs(p);
      podsDiv.appendChild(div);
    });
    try{
      const evs = await (await fetch('/api/events?namespace='+encodeURIComponent(st.namespace)+'&khcheck='+encodeURIComponent(name))).json();
      const eventsDiv=document.getElementById('events');
      eventsDiv.innerHTML='';
      if(evs.length===0){eventsDiv.textContent='No events found';}
      evs.forEach(ev=>{
        const div=document.createElement('div');
        div.className='p-1 border-b border-gray-200 dark:border-gray-700 text-gray-900 dark:text-gray-100';
        const ts=ev.lastTimestamp?new Date(ev.lastTimestamp*1000).toLocaleString()+': ':'';
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
<body class="flex flex-col min-h-screen font-sans text-gray-900 dark:text-gray-100 bg-gradient-to-b from-gray-100 to-gray-300 dark:from-gray-800 dark:to-gray-900">
<header class="flex items-center justify-between bg-gradient-to-r from-blue-600 to-indigo-600 text-white p-4 shadow-lg">
  <div class="flex items-center">
    <img src="/static/logo-square.png" alt="Kuberhealthy logo" class="h-8 w-8 mr-2" />
    <h1 class="text-lg font-semibold m-0">Kuberhealthy Status</h1>
  </div>
  <button id="themeToggle" class="text-sm px-2 py-1 rounded bg-white/20" onclick="setTheme(document.documentElement.classList.contains('dark')?'light':'dark')">🌙</button>
</header>
<div id="main" class="flex flex-1 overflow-hidden">
  <div id="menu" class="w-64 border-r overflow-y-auto bg-gradient-to-b from-gray-50 to-gray-200 dark:from-gray-900 dark:to-gray-700 shadow-inner"></div>
  <div id="content" class="flex-1 p-4 overflow-y-auto bg-gradient-to-b from-white to-gray-50 dark:from-gray-800 dark:to-gray-900"><h2 class="text-2xl font-bold">Select a check</h2></div>
</div>
<footer class="text-center p-2 bg-gradient-to-r from-gray-100 to-gray-200 dark:from-gray-800 dark:to-gray-700 shadow-inner">Powered by <a href="https://kuberhealthy.github.io/kuberhealthy/" class="text-blue-600 hover:underline">Kuberhealthy</a> • <a href="https://github.com/kuberhealthy/kuberhealthy/blob/main/docs/README.md" target="_blank" rel="noopener" class="text-blue-600 hover:underline">Documentation</a> • <a href="https://github.com/kuberhealthy/kuberhealthy" class="text-blue-600 hover:underline">Source code</a></footer>
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

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./assets"))))

	mux.HandleFunc("/api/run", func(w http.ResponseWriter, r *http.Request) {
		if err := runCheckHandler(w, r); err != nil {
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

// runCheckHandler triggers an immediate run of a check.
func runCheckHandler(w http.ResponseWriter, r *http.Request) error {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}
	khcheck := r.URL.Query().Get("khcheck")
	namespace := r.URL.Query().Get("namespace")
	if khcheck == "" || namespace == "" {
		w.WriteHeader(http.StatusBadRequest)
		return fmt.Errorf("missing parameters")
	}
	if Globals.khClient == nil || Globals.kh == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kuberhealthy not initialized")
	}
	nn := types.NamespacedName{Namespace: namespace, Name: khcheck}
	check, err := khapi.GetCheck(r.Context(), Globals.khClient, nn)
	if err != nil {
		if apierrors.IsNotFound(err) {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
		return err
	}
	if check.CurrentUUID() != "" {
		w.WriteHeader(http.StatusConflict)
		return fmt.Errorf("check already running")
	}
	if err := Globals.kh.StartCheck(check); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	// reset the check ticker so the next automatic run starts from now
	check.Status.LastRunUnix = time.Now().Unix()
	if err := khapi.UpdateCheck(r.Context(), Globals.khClient, check); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return fmt.Errorf("failed to update last run time: %w", err)
	}
	w.WriteHeader(http.StatusAccepted)
	return nil
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
	if Globals.khClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	nn := types.NamespacedName{Namespace: namespace, Name: khcheck}
	if _, err := khapi.GetCheck(r.Context(), Globals.khClient, nn); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return err
	}
	pods := &v1.PodList{}
	if err := Globals.khClient.List(r.Context(), pods, client.InNamespace(namespace), client.MatchingLabels{"khcheck": khcheck}); err != nil {
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
	if Globals.kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	fs := fmt.Sprintf("involvedObject.name=%s,involvedObject.kind=KuberhealthyCheck", khcheck)
	evList, err := Globals.kubeClient.CoreV1().Events(namespace).List(r.Context(), metav1.ListOptions{FieldSelector: fs})
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
	if Globals.khClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	pod := &v1.Pod{}
	if err := Globals.khClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: podName}, pod); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return err
	}
	if pod.Labels["khcheck"] != khcheck {
		w.WriteHeader(http.StatusForbidden)
		return fmt.Errorf("pod not part of khcheck")
	}
	nn := types.NamespacedName{Namespace: namespace, Name: khcheck}
	if _, err := khapi.GetCheck(r.Context(), Globals.khClient, nn); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return err
	}
	if Globals.kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	req := Globals.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{})
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
	if Globals.khClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	pod := &v1.Pod{}
	if err := Globals.khClient.Get(r.Context(), client.ObjectKey{Namespace: namespace, Name: podName}, pod); err != nil {
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
	nn := types.NamespacedName{Namespace: namespace, Name: khcheck}
	if _, err := khapi.GetCheck(r.Context(), Globals.khClient, nn); err != nil {
		w.WriteHeader(http.StatusNotFound)
		return err
	}
	if Globals.kubeClient == nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return fmt.Errorf("kubernetes client not initialized")
	}
	req := Globals.kubeClient.CoreV1().Pods(namespace).GetLogs(podName, &v1.PodLogOptions{Follow: true})
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
	validateUsingRequestHeaderFunc = validateUsingRequestHeader
	storeCheckStateFunc            = storeCheckState
)

// prometheusMetricsHandler is a handler for all prometheus metrics requests
func prometheusMetricsHandler(w http.ResponseWriter, r *http.Request) error {
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

	// Validate request using the kh-run-uuid header.
	log.Println("webserver:", requestID, "validating external check status report from run uuid:", r.Header.Get("kh-run-uuid"))
	podReport, reportValidated, err := validateUsingRequestHeaderFunc(ctx, r)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(err.Error()))
		log.Println("webserver:", requestID, "Failed to look up khcheck by its kh-run-uuid header:", r.Header.Get("kh-run-uuid"), err)
		return nil
	}
	if !reportValidated {
		w.WriteHeader(http.StatusBadRequest)
		log.Println("webserver:", requestID, "Request missing kh-run-uuid header")
		return nil
	}
	log.Println("webserver:", requestID, "Reporting check is", podReport.Name, "in namespace", podReport.Namespace)

	// append pod info to request id for easy check tracing in logs
	requestID = requestID + " (" + podReport.Namespace + "/" + podReport.Name + ")"

	// fetch the khcheck for event recording when a client is available
	var khCheck *khapi.KuberhealthyCheck
	if Globals.khClient != nil {
		nn := types.NamespacedName{Name: podReport.Name, Namespace: podReport.Namespace}
		var err error
		khCheck, err = khapi.GetCheck(ctx, Globals.khClient, nn)
		if err != nil {
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
	details := &khapi.KuberhealthyCheckStatus{}
	details.Errors = state.Errors
	details.OK = state.OK
	// details.RunDuration = checkRunDuration
	details.Namespace = podReport.Namespace
	// clear the UUID so that the scheduler can start the next run
	details.CurrentUUID = ""
	// since the check is validated, we can proceed to update the status now
	log.Println("webserver:", requestID, "Setting check with name", podReport.Name, "in namespace", podReport.Namespace, "to 'OK' state:", details.OK, "uuid", details.CurrentUUID)
	err = storeCheckStateFunc(Globals.khClient, podReport.Name, podReport.Namespace, details)
	if err != nil {
		if khCheck != nil && Globals.kh != nil && Globals.kh.Recorder != nil {
			Globals.kh.Recorder.Eventf(khCheck, v1.EventTypeWarning, "CheckReportFailed", "failed to store check state: %v", err)
		}
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("webserver:", requestID, "failed to store check state for %s: %w", podReport.Name, err)
		return fmt.Errorf("failed to store check state for %s: %w", podReport.Name, err)
	}
	log.WithFields(log.Fields{
		"namespace": podReport.Namespace,
		"name":      podReport.Name,
		"ok":        state.OK,
		"errors":    state.Errors,
	}).Info("checker pod reported")

	if khCheck != nil && Globals.kh != nil && Globals.kh.Recorder != nil {
		if state.OK {
			Globals.kh.Recorder.Event(khCheck, v1.EventTypeNormal, "CheckReported", "check reported OK")
		} else {
			Globals.kh.Recorder.Eventf(khCheck, v1.EventTypeWarning, "CheckReported", strings.Join(state.Errors, "; "))
		}
	}

	// write ok back to caller
	w.WriteHeader(http.StatusOK)
	log.Println("webserver:", requestID, "Request completed successfully.")
	return nil
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
	if Globals.khClient == nil {
		state.OK = false
		state.AddError("kubernetes client not initialized")
		return state
	}

	ctx := context.Background()
	uList := &unstructured.UnstructuredList{}
	uList.SetGroupVersionKind(khapi.GroupVersion.WithKind("KuberhealthyCheckList"))
	var opts []client.ListOption
	if len(namespaces) == 1 {
		opts = append(opts, client.InNamespace(namespaces[0]))
	}
	if err := Globals.khClient.List(ctx, uList, opts...); err != nil {
		state.OK = false
		state.AddError(err.Error())
		return state
	}

	for _, u := range uList.Items {
		if len(namespaces) > 1 && !containsString(u.GetNamespace(), namespaces) {
			continue
		}

		runInterval := runIntervalForCheck(&u)

		var check khapi.KuberhealthyCheck
		if err := runtime.DefaultUnstructuredConverter.FromUnstructured(u.Object, &check); err != nil {
			log.Errorf("failed to convert check %s/%s: %v", u.GetNamespace(), u.GetName(), err)
			continue
		}
		check.EnsureCreationTimestamp()

		status := health.CheckDetail{KuberhealthyCheckStatus: check.Status}
		if status.Namespace == "" {
			status.Namespace = check.Namespace
		}
		if check.CurrentUUID() != "" {
			pods := &v1.PodList{}
			err := Globals.khClient.List(ctx, pods,
				client.InNamespace(check.Namespace),
				client.MatchingLabels{"kh-run-uuid": check.CurrentUUID()},
			)
			if err == nil && len(pods.Items) > 0 {
				status.PodName = pods.Items[0].Name
			}
		} else if check.Status.LastRunUnix != 0 {
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

// validateUsingRequestHeader ensures the request includes a valid kh-run-uuid header and
// returns information about the associated KuberhealthyCheck.
func validateUsingRequestHeader(ctx context.Context, r *http.Request) (PodReportInfo, bool, error) {
	var info PodReportInfo
	uuid := r.Header.Get("kh-run-uuid")
	if uuid == "" {
		return info, false, nil
	}
	check, err := findCheckByUUID(ctx, uuid)
	if err != nil {
		return info, true, fmt.Errorf("failed to lookup khcheck for uuid %s: %w", uuid, err)
	}
	if check == nil {
		return info, true, fmt.Errorf("run uuid %s is invalid or expired", uuid)
	}
	info.Name = check.Name
	info.Namespace = check.Namespace
	info.UUID = uuid
	return info, true, nil
}

// findCheckByUUID searches for a KuberhealthyCheck whose status CurrentUUID matches the provided uuid.
func findCheckByUUID(ctx context.Context, uuid string) (*khapi.KuberhealthyCheck, error) {
	if Globals.khClient == nil {
		return nil, fmt.Errorf("kubernetes client not initialized")
	}
	list := &khapi.KuberhealthyCheckList{}
	if err := Globals.khClient.List(ctx, list); err != nil {
		return nil, err
	}
	for i := range list.Items {
		if list.Items[i].CurrentUUID() == uuid {
			return &list.Items[i], nil
		}
	}
	return nil, nil
}

// storeCheckState stores the check status on its cluster CRD
func storeCheckState(c client.Client, checkName string, checkNamespace string, khcheck *khapi.KuberhealthyCheckStatus) error {
	// ensure the CRD resource exists
	if err := ensureCheckResourceExists(c, checkName, checkNamespace); err != nil {
		return err
	}

	// put the status on the CRD from the check
	err := setCheckStatus(c, checkName, checkNamespace, khcheck)

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
		err = setCheckStatus(c, checkName, checkNamespace, khcheck)

		// count how many times we've retried
		tries++
	}

	return err
}

// ensureCheckResourceExists ensures that the khcheck resource exists so that status can be updated
func ensureCheckResourceExists(c client.Client, checkName string, checkNamespace string) error {
	if c == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ctx := context.Background()
	nn := types.NamespacedName{Name: checkName, Namespace: checkNamespace}
	khCheck, err := khapi.GetCheck(ctx, c, nn)
	if err != nil {
		if apierrors.IsNotFound(err) {
			khCheck = &khapi.KuberhealthyCheck{ObjectMeta: metav1.ObjectMeta{Name: checkName, Namespace: checkNamespace}}
			if err := khapi.CreateCheck(ctx, c, khCheck); err != nil {
				return fmt.Errorf("failed to create khcheck %s/%s: %w", checkNamespace, checkName, err)
			}
			return nil
		}
		return fmt.Errorf("failed to get khcheck %s/%s: %w", checkNamespace, checkName, err)
	}
	return nil
}

// setCheckStatus sets the status on the khcheck custom resource
func setCheckStatus(c client.Client, checkName string, checkNamespace string, khcheck *khapi.KuberhealthyCheckStatus) error {
	if c == nil {
		return fmt.Errorf("kubernetes client not initialized")
	}

	ctx := context.Background()
	nn := types.NamespacedName{Name: checkName, Namespace: checkNamespace}
	khCheck, err := khapi.GetCheck(ctx, c, nn)
	if err != nil {
		return fmt.Errorf("failed to get khcheck: %w", err)
	}

	khCheck.Status = *khcheck
	if khCheck.Status.Namespace == "" {
		khCheck.Status.Namespace = checkNamespace
	}

	if err := khapi.UpdateCheck(ctx, c, khCheck); err != nil {
		return fmt.Errorf("failed to update khcheck status: %w", err)
	}

	return nil
}
