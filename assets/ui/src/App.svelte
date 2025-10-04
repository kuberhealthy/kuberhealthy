<script>
  import { onMount } from 'svelte';
  import Menu from './Menu.svelte';
  import Home from './Home.svelte';
  import Check from './Check.svelte';
  import { checks, currentCheck, refreshStatus } from './stores';

  let initialNamespace = '';
  let initialCheck = '';
  let pendingQuerySelection = false;
  let allowURLSync = false;

  /**
   * findCheckFromQuery locates the check name that matches the query parameters.
   * It returns an empty string when no matching check is found.
   */
  function findCheckFromQuery(targetNamespace, targetName, allChecks){
    if(!targetName){
      return '';
    }

    const direct = allChecks[targetName];
    if(direct && (!targetNamespace || direct.namespace === targetNamespace)){
      return targetName;
    }

    for(const [name, detail] of Object.entries(allChecks)){
      if(!detail){
        continue;
      }

      const segments = name.split('/');
      const lastSegment = segments[segments.length - 1];
      if(lastSegment !== targetName){
        continue;
      }

      if(targetNamespace && detail.namespace !== targetNamespace){
        continue;
      }

      return name;
    }

    return '';
  }

  async function refresh(){
    try{
      const resp = await fetch('/json');
      const data = await resp.json();
      checks.set(data.CheckDetails || {});
    }catch(e){ console.error('failed to fetch status', e); }
  }

  refreshStatus.set(refresh);

  onMount(() => {
    const params = new URLSearchParams(window.location.search);
    const namespaceFromQuery = params.get('namespace') || '';
    const checkFromQuery = params.get('check') || '';

    if(checkFromQuery){
      currentCheck.set(checkFromQuery);
      initialNamespace = namespaceFromQuery;
      initialCheck = checkFromQuery;
      pendingQuerySelection = true;
    }

    refresh();
    const timer = setInterval(refresh, 5000);
    allowURLSync = true;
    return () => clearInterval(timer);
  });

  $: if (pendingQuerySelection){
    const resolved = findCheckFromQuery(initialNamespace, initialCheck, $checks);
    if(resolved){
      currentCheck.set(resolved);
      pendingQuerySelection = false;
    }
  }

  $: if (allowURLSync && $currentCheck) {
    const st = $checks[$currentCheck];
    if (st) {
      const params = new URLSearchParams();
      params.set('namespace', st.namespace);
      params.set('check', $currentCheck);
      history.replaceState(null, '', '?' + params.toString());
    }
  }

  $: if (allowURLSync && !$currentCheck) {
    history.replaceState(null, '', location.pathname);
  }

</script>

<header class="flex items-center bg-white text-purple-700 p-4 shadow-lg">
  <div class="flex items-center cursor-pointer" role="button" tabindex="0" on:click={() => currentCheck.set('')} on:keydown={(e) => e.key === 'Enter' && currentCheck.set('')}>
    <img src="/static/logo-square.png" alt="Kuberhealthy logo" class="h-10 w-10 mr-2" />
    <h1 class="text-lg font-semibold m-0">Kuberhealthy Status</h1>
  </div>
</header>
<div id="main" class="flex flex-1 overflow-hidden">
  <Menu />
  <div id="content" class="flex-1 p-4 overflow-y-auto bg-gradient-to-b from-white to-gray-50 dark:from-gray-800 dark:to-gray-900">
    {#if $currentCheck}
      <Check name={$currentCheck} />
    {:else}
      <Home />
    {/if}
  </div>
</div>
<footer class="text-center p-2 bg-gradient-to-r from-gray-100 to-gray-200 dark:from-gray-800 dark:to-gray-700 shadow-inner">
  Powered by <a href="https://kuberhealthy.github.io/kuberhealthy/" class="text-blue-600 hover:underline">Kuberhealthy</a> •
  <a href="https://github.com/kuberhealthy/kuberhealthy/blob/main/docs/README.md" target="_blank" rel="noopener" class="text-blue-600 hover:underline">Documentation</a> •
  <a href="https://github.com/kuberhealthy/kuberhealthy" class="text-blue-600 hover:underline">Source code</a>
</footer>
