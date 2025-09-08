<script>
  import { onMount, onDestroy } from 'svelte';
  import { checks } from './stores';
  import { formatDuration } from './util';

  export let name = '';
  let st;
  let nextRun = '';
  let failIn = '';
  let events = [];
  let podInfo = '';
  let logText = '';
  let eventsTimer;
  let countdownTimer;
  let logStreamAbort;

  $: st = $checks[name];

  function clearTimers(){
    if(eventsTimer){ clearInterval(eventsTimer); eventsTimer = null; }
    if(countdownTimer){ clearInterval(countdownTimer); countdownTimer = null; }
    if(logStreamAbort){ logStreamAbort.abort(); logStreamAbort = null; }
  }

  onMount(setup);
  onDestroy(clearTimers);

  $: if (st) { setup(); }

  function setup(){
    clearTimers();
    if(!st){ return; }
    if(st.podName && st.timeoutSeconds){
      const failUnix = st.lastRunUnix + st.timeoutSeconds;
      updateFail(failUnix);
      countdownTimer = setInterval(() => updateFail(failUnix), 1000);
    } else if(st.nextRunUnix && !st.podName){
      updateNextRun(st.nextRunUnix);
      countdownTimer = setInterval(() => updateNextRun(st.nextRunUnix), 1000);
    }
    if(st.podName){
      loadLogs();
    } else {
      podInfo = '';
      logText = '';
    }
    loadEvents();
    eventsTimer = setInterval(loadEvents, 5000);
  }

  function updateNextRun(unix){
    nextRun = formatDuration(unix * 1000 - Date.now());
  }

  function updateFail(unix){
    failIn = formatDuration(unix * 1000 - Date.now());
  }

  async function loadEvents(){
    try{
      const url = '/api/events?namespace=' + encodeURIComponent(st.namespace) + '&khcheck=' + encodeURIComponent(name) + '&t=' + Date.now();
      events = await (await fetch(url)).json();
      events.sort((a,b)=>(a.lastTimestamp||0)-(b.lastTimestamp||0));
    }catch(e){ console.error(e); }
  }

  async function loadLogs(){
    try{
      const params = 'namespace=' + encodeURIComponent(st.namespace) + '&khcheck=' + encodeURIComponent(name) + '&pod=' + encodeURIComponent(st.podName);
      const res = await (await fetch('/api/logs?' + params)).json();
      podInfo = 'Started: ' + (res.startTime ? new Date(res.startTime*1000).toLocaleString() : '') +
        ' Duration: ' + res.durationSeconds + 's Phase: ' + res.phase +
        (res.labels ? ' Labels: ' + Object.entries(res.labels).map(([k,v])=>k+':' + v).join(', ') : '');
      logText = res.logs || '';
      if(res.phase === 'Running'){
        streamLogs('/api/logs/stream?' + params);
      }
    }catch(e){ console.error(e); }
  }

  async function streamLogs(url){
    try{
      logStreamAbort = new AbortController();
      const resp = await fetch(url, {signal: logStreamAbort.signal});
      if(!resp.body){ return; }
      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      while(true){
        const {value, done} = await reader.read();
        if(done){ break; }
        logText += decoder.decode(value);
      }
    }catch(e){ if(e.name !== 'AbortError'){ console.error(e); } }
  }

  async function runNow(){
    try{
      await fetch('/api/run?namespace=' + encodeURIComponent(st.namespace) + '&khcheck=' + encodeURIComponent(name), {method:'POST'});
    }catch(e){ console.error(e); }
  }
</script>

<div>
  <h2 class="text-2xl font-bold mb-4">{name}</h2>
  <div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4">
    <h3 class="text-xl font-semibold mb-2">Overview</h3>
    {#if st}
      <p class="mb-2"><span class="font-semibold">Status:</span> <span class={st.podName ? 'text-blue-600' : st.ok ? 'text-green-600 font-bold' : 'text-red-600 font-bold'}>{st.podName ? 'Running' : st.ok ? 'OK' : 'ERROR'}</span></p>
      <p class="mb-2"><span class="font-semibold">Namespace:</span> {st.namespace}</p>
      {#if st.runIntervalSeconds}
        <p class="mb-2"><span class="font-semibold">Run interval:</span> {formatDuration(st.runIntervalSeconds*1000)}</p>
      {/if}
      {#if st.timeoutSeconds}
        <p class="mb-2"><span class="font-semibold">Timeout:</span> {formatDuration(st.timeoutSeconds*1000)}</p>
      {/if}
      {#if st.podName}
        <p class="mb-2"><span class="font-semibold">Pod:</span> {st.podName}</p>
        {#if st.timeoutSeconds}
          <p class="mb-2"><span class="font-semibold">Fail in:</span> {failIn}</p>
        {/if}
      {:else if st.nextRunUnix}
        <p class="mb-2 flex items-center gap-2"><span class="font-semibold">Next run in:</span> {nextRun}<button class="px-2 py-1 text-xs bg-blue-600 text-white rounded" on:click={runNow}>Run now</button></p>
      {/if}
      {#if st.lastRunUnix}
        <p class="mb-2"><span class="font-semibold">Last run:</span> {new Date(st.lastRunUnix*1000).toLocaleString()}</p>
      {/if}
      {#if st.errors && st.errors.length}
        <div class="mb-2"><span class="font-semibold text-red-600">Errors:</span><ul class="list-disc list-inside">{#each st.errors as e}<li>{e}</li>{/each}</ul></div>
      {/if}
    {/if}
  </div>
  <div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4">
    <h3 class="text-xl font-semibold mb-2">Events</h3>
    {#if events.length === 0}
      <div>No events found</div>
    {:else}
      {#each events.slice(-10) as ev}
        <div class="p-1 border-b border-gray-200 dark:border-gray-700">
          {ev.lastTimestamp ? new Date(ev.lastTimestamp*1000).toLocaleString()+': ' : ''}[{ev.type}] {ev.reason} - {ev.message}
        </div>
      {/each}
      {#if events.length > 10}
        <details class="mt-2">
          <summary class="cursor-pointer text-blue-600">Show {events.length - 10} more</summary>
          {#each events.slice(0, -10) as ev}
            <div class="p-1 border-b border-gray-200 dark:border-gray-700">
              {ev.lastTimestamp ? new Date(ev.lastTimestamp*1000).toLocaleString()+': ' : ''}[{ev.type}] {ev.reason} - {ev.message}
            </div>
          {/each}
        </details>
      {/if}
    {/if}
  </div>
  <div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4">
    <h3 class="text-xl font-semibold mb-2">Pod Details</h3>
    <div>{podInfo}</div>
  </div>
  <div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4">
    <h3 class="text-xl font-semibold mb-2">Logs</h3>
    <pre class="whitespace-pre-wrap bg-gray-100 dark:bg-gray-800 p-4 rounded shadow-inner">{logText}</pre>
  </div>
</div>
