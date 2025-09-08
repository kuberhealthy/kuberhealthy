<script>
  import { onMount } from 'svelte';
  import Menu from './Menu.svelte';
  import Home from './Home.svelte';
  import Check from './Check.svelte';
  import { checks, currentCheck } from './stores';

  let theme = 'dark';

  function setTheme(t){
    theme = t;
    if(t === 'dark'){ document.documentElement.classList.add('dark'); }
    else{ document.documentElement.classList.remove('dark'); }
    document.documentElement.setAttribute('data-bs-theme', t);
    localStorage.setItem('kh-theme', t);
  }

  function toggleTheme(){
    setTheme(theme === 'dark' ? 'light' : 'dark');
  }

  function initTheme(){
    const saved = localStorage.getItem('kh-theme') || 'dark';
    setTheme(saved);
  }

  async function refresh(){
    try{
      const resp = await fetch('/json');
      const data = await resp.json();
      checks.set(data.CheckDetails || {});
    }catch(e){ console.error('failed to fetch status', e); }
  }

  onMount(() => {
    initTheme();
    const params = new URLSearchParams(window.location.search);
    const hashParams = new URLSearchParams(window.location.hash.slice(1));
    const check = params.get('check') || hashParams.get('check');
    if(check){ currentCheck.set(check); }
    refresh();
    const timer = setInterval(refresh, 5000);
    return () => clearInterval(timer);
  });

  $: if ($currentCheck) {
    const st = $checks[$currentCheck];
    if (st) {
      const params = new URLSearchParams();
      params.set('namespace', st.namespace);
      params.set('check', $currentCheck);
      history.replaceState(null, '', '?' + params.toString());
      location.hash = 'check=' + encodeURIComponent($currentCheck);
    }
  } else {
    history.replaceState(null, '', location.pathname);
    location.hash = '';
  }

</script>

<header class="flex items-center justify-between bg-gradient-to-r from-blue-600 to-indigo-600 text-white p-4 shadow-lg">
  <div class="flex items-center cursor-pointer" role="button" tabindex="0" on:click={() => currentCheck.set('')} on:keydown={(e) => e.key === 'Enter' && currentCheck.set('')}>
    <img src="/static/logo-square.png" alt="Kuberhealthy logo" class="h-8 w-8 mr-2" />
    <h1 class="text-lg font-semibold m-0">Kuberhealthy Status</h1>
  </div>
  <button class="text-sm px-2 py-1 rounded bg-white/20" on:click={toggleTheme}>{theme === 'dark' ? '☀️' : '🌙'}</button>
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
