<script>
  import { onMount } from 'svelte';
  import { symbolsIndex, humanReadableSymbol } from '../util/constants.js';
  import { barRanges } from '../stores/barRanges.js';

  export let active = false;

  onMount(async () => {
    await barRanges.load();
  });

  export let closePopup = () => {};
</script>

{#if $barRanges.length && active}
  <!-- Overlay -->
  <div class="overlay fixed inset-0 bg-black bg-opacity-50 flex justify-center items-center min-w-80" on:click={closePopup}>
    <!-- Popup Content -->
    <div class="popup bg-white p-5 rounded-lg shadow-lg" onclick={event => event.stopPropagation()}>
      <h2 class="text-xl font-semibold mb-4">Available Date Ranges</h2>
      <table class="border-collapse border border-black">
        <thead>
          <tr class="border-b border-black">
            <th class="border border-black p-2">Symbol</th>
            <th class="border border-black p-2">First Date</th>
            <th class="border border-black p-2">Latest Date</th>
          </tr>
        </thead>
        <tbody>
          {#each $barRanges as { symbol_id, first_date, last_date }}
            <tr>
              <td class="border border-black p-2">{humanReadableSymbol[symbolsIndex[symbol_id]]}</td>
              <td class="border border-black p-2">{first_date}</td>
              <td class="border border-black p-2">{last_date}</td>
            </tr>
          {/each}
        </tbody>
      </table>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    top: 0;
    left: 0;
    right: 0;
    bottom: 0;
    background-color: rgba(0, 0, 0, 0.5);
    display: flex;
    justify-content: center;
    align-items: center;
    z-index: 1;
  }
  .popup {
    background-color: white;
    padding: 20px;
    border-radius: 5px;
    box-shadow: 0 5px 15px rgba(0, 0, 0, 0.3);
  }
  ul {
    list-style-type: none;
    padding: 0;
  }
  button {
    margin-top: 20px;
    padding: 10px 20px;
    border: none;
    border-radius: 5px;
    cursor: pointer;
  }
</style>
