<script>
  import { checks, currentCheck } from './stores';

  function select(name){
    currentCheck.set(name);
  }

  function statusBadge(st){
    // Fall back to an unknown badge when the status payload is missing.
    if(!st){
      return {
        label: 'Unknown',
        classes: 'bg-gray-200 text-gray-700 dark:bg-gray-700 dark:text-gray-200'
      };
    }

    // Highlight running checks so operators can see active work quickly.
    if(st.podName){
      return {
        label: 'Running',
        classes: 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200'
      };
    }

    // Healthy checks earn the OK badge for quick scanning.
    if(st.ok){
      return {
        label: 'OK',
        classes: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200'
      };
    }

    // Anything else is treated as failing for triage purposes.
    return {
      label: 'Failing',
      classes: 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200'
    };
  }

  $: grouped = (() => {
    const groups = {};
    for (const [name, st] of Object.entries($checks)){
      const ns = st.namespace || '';
      if(!groups[ns]){ groups[ns] = []; }
      const badge = statusBadge(st);
      groups[ns].push({name, st, badge});
    }
    return groups;
  })();
</script>

<div class="w-64 border-r overflow-y-auto bg-gradient-to-b from-gray-50 to-gray-200 dark:from-gray-900 dark:to-gray-700 shadow-inner">
  <div class="p-3 font-semibold text-gray-700 dark:text-gray-300">Checks by Namespace</div>
  {#each Object.keys(grouped).sort() as ns}
    <div class="px-3 py-1 text-xs font-semibold text-gray-600 dark:text-gray-400 uppercase">{ns}</div>
    {#each grouped[ns].sort((a,b)=>a.name.localeCompare(b.name)) as item}
      <div class="p-2 cursor-pointer hover:bg-gray-100 dark:hover:bg-gray-800 text-gray-900 dark:text-gray-100 flex items-start gap-3" class:bg-white={item.name === $currentCheck} class:dark:bg-gray-600={item.name === $currentCheck} class:font-bold={item.name === $currentCheck} role="button" tabindex="0" on:click={() => select(item.name)} on:keydown={(e) => e.key === 'Enter' && select(item.name)}>
        <div class="flex-1 min-w-0">
          <div class="flex items-center gap-2">
            <span>{item.st.podName ? '⏳' : item.st.ok ? '✅' : '❌'}</span>
            <span class="truncate">{item.name}</span>
          </div>
          {#if !item.st.ok && item.st.errors && item.st.errors.length}
            <div class="text-xs text-red-600 dark:text-red-300 truncate mt-1">{item.st.errors[0]}</div>
          {/if}
        </div>
        <span class={`ml-auto px-2 py-0.5 text-xs font-semibold rounded-full whitespace-nowrap ${item.badge.classes}`}>
          {item.badge.label}
        </span>
      </div>
    {/each}
  {/each}
</div>
