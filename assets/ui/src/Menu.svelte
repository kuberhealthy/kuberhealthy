<script>
  import { checks, currentCheck } from './stores';
  import { formatDuration } from './util';

  function select(name){
    currentCheck.set(name);
  }

  $: grouped = (() => {
    const groups = {};
    for (const [name, st] of Object.entries($checks)){
      const ns = st.namespace || '';
      if(!groups[ns]){ groups[ns] = []; }
      groups[ns].push({name, st});
    }
    return groups;
  })();
</script>

<div class="w-64 border-r overflow-y-auto bg-gradient-to-b from-gray-50 to-gray-200 dark:from-gray-900 dark:to-gray-700 shadow-inner">
  <div class="p-3 font-semibold text-gray-700 dark:text-gray-300">Checks by Namespace</div>
  {#each Object.keys(grouped).sort() as ns}
    <div class="px-3 py-1 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">{ns}</div>
    {#each grouped[ns].sort((a,b)=>a.name.localeCompare(b.name)) as item}
      <div class="p-2 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-900 dark:text-gray-100" class:bg-white={item.name === $currentCheck} class:dark:bg-gray-600={item.name === $currentCheck} class:font-bold={item.name === $currentCheck} role="button" tabindex="0" on:click={() => select(item.name)} on:keydown={(e) => e.key === 'Enter' && select(item.name)}>
        {item.st.podName ? '⏳' : item.st.ok ? '✅' : '❌'} {item.name}
        {#if !item.st.ok && item.st.errors && item.st.errors.length}
          - {item.st.errors[0]}
        {/if}
        {#if item.st.nextRunUnix}
          ({formatDuration(item.st.nextRunUnix * 1000 - Date.now())})
        {/if}
      </div>
    {/each}
  {/each}
</div>
