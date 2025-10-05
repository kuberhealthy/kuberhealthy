<script>
  import { onMount, onDestroy, tick } from 'svelte';
  import { checks, refreshStatus } from './stores';
  import { formatDuration } from './util';

  export let name = '';
  let st;
  let nextRun = '';
  let failIn = '';
  let events = [];
  let eventsText = '';
  let logsURL = '';
  let streamURL = '';
  let isRunning = false;
  let eventsTimer;
  let eventsBox;
  let countdownTimer;
  let runButtonTimer;
  let showRunButton = true;
  let countdownAllowsRunButton = true;
  let triggerRefresh = async () => {};
  let refreshUnsubscribe = () => {};
  let lastRefreshUnix = null;
  let lastRefreshTime = 0;

  $: st = $checks[name];

  function clearTimers(){
    if(eventsTimer){ clearInterval(eventsTimer); eventsTimer = null; }
    if(countdownTimer){ clearInterval(countdownTimer); countdownTimer = null; }
    if(runButtonTimer){ clearTimeout(runButtonTimer); runButtonTimer = null; }
  }

  onMount(() => {
    refreshUnsubscribe = refreshStatus.subscribe(fn => {
      triggerRefresh = typeof fn === 'function' ? fn : async () => {};
    });
    setup();
  });

  onDestroy(() => {
    clearTimers();
    refreshUnsubscribe();
  });

  $: if (st) { setup(); }

  function setup(){
    clearTimers();
    lastRefreshUnix = null;
    lastRefreshTime = 0;
    if(!st){ return; }
    showRunButton = true;
    if(st.podName && st.timeoutSeconds){
      const failUnix = st.lastRunUnix + st.timeoutSeconds;
      updateFail(failUnix);
      countdownTimer = setInterval(() => updateFail(failUnix), 1000);
    } else if(st.nextRunUnix && !st.podName){
      updateNextRun(st.nextRunUnix);
      countdownTimer = setInterval(() => updateNextRun(st.nextRunUnix), 1000);
    }
    if(st.podName){
      const params = 'namespace=' + encodeURIComponent(st.namespace) + '&khcheck=' + encodeURIComponent(name) + '&pod=' + encodeURIComponent(st.podName);
      logsURL = '/api/logs?' + params + '&format=text';
      streamURL = '/api/logs/stream?' + params;
      isRunning = true;
    } else {
      logsURL = '';
      streamURL = '';
      isRunning = false;
    }
    loadEvents();
    eventsTimer = setInterval(loadEvents, 5000);
  }

  // updateNextRun updates the text shown for the next run countdown and triggers a refresh when the countdown completes.
  function updateNextRun(unix){
    const diff = unix * 1000 - Date.now();
    if (diff <= 0) {
      nextRun = 'Starting check run...';
      countdownAllowsRunButton = false;
      const now = Date.now();
      if (unix !== lastRefreshUnix || now - lastRefreshTime > 5000) {
        lastRefreshUnix = unix;
        lastRefreshTime = now;
        Promise.resolve()
          .then(() => triggerRefresh())
          .catch((err) => console.error('failed to refresh status', err));
      }
      return;
    }
    nextRun = formatDuration(diff);
    countdownAllowsRunButton = true;
    lastRefreshUnix = null;
  }

  function updateFail(unix){
    failIn = formatDuration(unix * 1000 - Date.now());
  }

  async function loadEvents(){
    try{
      const url = '/api/events?namespace=' + encodeURIComponent(st.namespace) + '&khcheck=' + encodeURIComponent(name) + '&t=' + Date.now();
      const nextEvents = await (await fetch(url)).json();
      nextEvents.sort((a,b)=>(a.lastTimestamp||0)-(b.lastTimestamp||0));
      events = nextEvents;
      eventsText = events
        .map(ev => `${ev.lastTimestamp ? new Date(ev.lastTimestamp * 1000).toLocaleString() + ': ' : ''}[${ev.type}] ${ev.reason} - ${ev.message}`)
        .join('\n');
      await tick();
      if (eventsBox) {
        eventsBox.scrollTop = eventsBox.scrollHeight;
      }
    }catch(e){ console.error(e); }
  }

  async function runNow(){
    try{
      showRunButton = false;
      if(runButtonTimer){ clearTimeout(runButtonTimer); }
      runButtonTimer = setTimeout(() => { showRunButton = true; runButtonTimer = null; }, 5000);
      await fetch('/api/run?namespace=' + encodeURIComponent(st.namespace) + '&khcheck=' + encodeURIComponent(name), {method:'POST'});
    }catch(e){ console.error(e); }
  }
</script>

<div class="flex flex-col flex-1 min-h-0">
  <h2 class="text-2xl font-bold mb-4">{name}</h2>
  <div class="mb-4 bg-white dark:bg-gray-900 rounded shadow p-4">
    <h3 class="text-xl font-semibold mb-2">Overview</h3>
    {#if st}
      <div class="flex flex-wrap items-start">
        <div class="flex-1 min-w-[260px] md:pr-4">
          <p class="mb-2"><span class="font-semibold">Status:</span>
            <span class={st.podName ? 'text-blue-600' : (st.ok ? 'text-green-600 font-bold' : 'text-red-600 font-bold')}>
              {st.podName ? 'Running' : (st.ok ? 'OK' : 'ERROR')}
            </span>
            {#if !st.ok && st.errors && st.errors.length}
              - {st.errors[0]}
            {/if}
          </p>
          <p class="mb-2"><span class="font-semibold">Namespace:</span> {st.namespace}</p>
          {#if st.runIntervalSeconds}
            <p class="mb-2"><span class="font-semibold">Run interval:</span> {formatDuration(st.runIntervalSeconds*1000)}</p>
          {/if}
          {#if st.timeoutSeconds}
            <p class="mb-2"><span class="font-semibold">Timeout:</span> {formatDuration(st.timeoutSeconds*1000)}</p>
          {/if}
          {#if st.podName}
            <p class="mb-2"><span class="font-semibold">Pod:</span> {st.podName}</p>
          {/if}
        </div>
        <div class="flex-1 min-w-[260px] mt-4 md:mt-0 md:pl-4">
          {#if st.podName}
            {#if st.timeoutSeconds}
              <p class="mb-2"><span class="font-semibold">Fail in:</span> {failIn}</p>
            {/if}
            <div class="mb-2 flex gap-4 items-center">
              <a class="px-2 py-1 text-xs bg-blue-600 text-white rounded" href={logsURL} target="_blank" rel="noopener noreferrer">Open logs (new tab)</a>
              {#if isRunning}
                <a class="px-2 py-1 text-xs bg-indigo-600 text-white rounded" href={streamURL} target="_blank" rel="noopener noreferrer">Open streaming logs (new tab)</a>
              {/if}
            </div>
          {:else if st.nextRunUnix}
            <p class="mb-2 flex items-center gap-2"><span class="font-semibold">Next run in:</span> {nextRun}
              {#if showRunButton && countdownAllowsRunButton}
                <button class="px-2 py-1 text-xs bg-blue-600 text-white rounded" on:click={runNow}>Run again now</button>
              {/if}
            </p>
          {:else}
            {#if showRunButton && countdownAllowsRunButton}
              <p class="mb-2 flex items-center gap-2">
                <button class="px-2 py-1 text-xs bg-blue-600 text-white rounded" on:click={runNow}>Run again now</button>
              </p>
            {/if}
          {/if}
          {#if st.lastRunUnix}
            <p class="mb-2"><span class="font-semibold">Last run:</span> {new Date(st.lastRunUnix*1000).toLocaleString()}</p>
          {/if}
        </div>
      </div>
      {#if st.errors && st.errors.length}
        <div class="mb-2"><span class="font-semibold text-red-600">Errors:</span><ul class="list-disc list-inside">{#each st.errors as e}<li>{e}</li>{/each}</ul></div>
      {/if}
    {/if}
  </div>
  <div class="flex-1 mb-4 bg-white dark:bg-gray-900 rounded shadow p-4 flex flex-col min-h-0">
    <h3 class="text-xl font-semibold mb-2">Events</h3>
    {#if events.length === 0}
      <div class="flex-1 flex items-center justify-center text-sm text-gray-500 dark:text-gray-400">No events found</div>
    {:else}
      <textarea
        class="flex-1 w-full min-h-0 resize-none font-mono text-sm bg-gray-100 dark:bg-gray-800 text-gray-900 dark:text-gray-100 border border-gray-300 dark:border-gray-700 rounded p-2"
        readonly
        bind:this={eventsBox}
        value={eventsText}
        spellcheck="false"
      ></textarea>
    {/if}
  </div>
</div>
