<script lang="ts">
import type { CalendarEvent } from "$lib/calendar";

interface Props {
  event: CalendarEvent;
}

let { event }: Props = $props();
</script>

<div
	class="truncate rounded-sm border-l-2 px-1.5 py-0.5 text-[11px] font-semibold"
	style="background-color: color-mix(in srgb, {event.color} 15%, transparent); border-color: {event.color}; color: {event.color}"
>
	{event.title}
</div>
