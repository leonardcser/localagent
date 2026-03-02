<script lang="ts">
import { Popover } from "bits-ui";
import { Icon } from "svelte-icons-pack";
import { FiRepeat, FiChevronDown, FiX } from "svelte-icons-pack/fi";

let {
  value = "",
  onchange,
}: {
  value: string;
  onchange: (rrule: string) => void;
} = $props();

let open = $state(false);

const WEEKDAY_LABELS = ["MO", "TU", "WE", "TH", "FR", "SA", "SU"];
const WEEKDAY_NAMES = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

const presets = [
  { label: "None", value: "" },
  { label: "Daily", value: "FREQ=DAILY" },
  { label: "Weekly", value: "FREQ=WEEKLY" },
  { label: "Monthly", value: "FREQ=MONTHLY" },
  { label: "Weekdays", value: "FREQ=WEEKLY;BYDAY=MO,TU,WE,TH,FR" },
];

let customMode = $state(false);
let freq = $state("WEEKLY");
let interval = $state(1);
let byDay = $state<string[]>([]);

function parseRrule(rrule: string) {
  freq = "WEEKLY";
  interval = 1;
  byDay = [];
  if (!rrule) return;
  for (const part of rrule.split(";")) {
    const [k, v] = part.split("=");
    if (k === "FREQ") freq = v;
    else if (k === "INTERVAL") interval = parseInt(v) || 1;
    else if (k === "BYDAY") byDay = v.split(",");
  }
}

function buildRrule(): string {
  let parts = [`FREQ=${freq}`];
  if (interval > 1) parts.push(`INTERVAL=${interval}`);
  if (freq === "WEEKLY" && byDay.length > 0)
    parts.push(`BYDAY=${byDay.join(",")}`);
  return parts.join(";");
}

function toggleDay(day: string) {
  if (byDay.includes(day)) {
    byDay = byDay.filter((d) => d !== day);
  } else {
    byDay = [...byDay, day];
  }
}

function applyCustom() {
  const rrule = buildRrule();
  onchange(rrule);
  open = false;
  customMode = false;
}

function humanReadable(rrule: string): string {
  if (!rrule) return "None";
  const preset = presets.find((p) => p.value === rrule);
  if (preset) return preset.label;

  let parts: Record<string, string> = {};
  for (const p of rrule.split(";")) {
    const [k, v] = p.split("=");
    parts[k] = v;
  }

  let base = "";
  const iv = parseInt(parts.INTERVAL || "1");
  switch (parts.FREQ) {
    case "DAILY":
      base = iv > 1 ? `Every ${iv} days` : "Daily";
      break;
    case "WEEKLY":
      base = iv > 1 ? `Every ${iv} weeks` : "Weekly";
      break;
    case "MONTHLY":
      base = iv > 1 ? `Every ${iv} months` : "Monthly";
      break;
    case "YEARLY":
      base = iv > 1 ? `Every ${iv} years` : "Yearly";
      break;
    default:
      return rrule;
  }

  if (parts.BYDAY) {
    const days = parts.BYDAY.split(",")
      .map((d) => WEEKDAY_NAMES[WEEKDAY_LABELS.indexOf(d)] ?? d)
      .join(", ");
    base += ` on ${days}`;
  }

  return base;
}
</script>

<Popover.Root bind:open>
	<Popover.Trigger
		class="flex items-center gap-1.5 rounded-lg border border-border bg-bg-tertiary px-2 py-1 text-[12px] text-text-primary outline-none hover:border-border-light focus:border-accent"
	>
		<span class="truncate">{humanReadable(value)}</span>
		{#if value}
			<button
				onclick={(e) => {
					e.stopPropagation();
					onchange("");
				}}
				class="text-text-muted hover:text-text-secondary"
			>
				<Icon src={FiX} size="10" />
			</button>
		{:else}
			<Icon src={FiChevronDown} size="11" className="text-text-muted" />
		{/if}
	</Popover.Trigger>
	<Popover.Portal>
		<Popover.Content
			class="z-50 min-w-48 rounded-lg border border-border bg-bg-secondary p-1 shadow-elevated"
			sideOffset={4}
		>
			{#if !customMode}
				{#each presets as preset}
					<button
						onclick={() => {
							onchange(preset.value);
							open = false;
						}}
						class="flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none hover:bg-overlay-light hover:text-text-primary
							{value === preset.value ? 'bg-overlay-light text-text-primary' : ''}"
					>
						{preset.label}
					</button>
				{/each}
				<div class="my-1 border-t border-border"></div>
				<button
					onclick={() => {
						parseRrule(value);
						customMode = true;
					}}
					class="flex w-full items-center gap-2 rounded-md px-2.5 py-1.5 text-[12px] text-text-secondary outline-none hover:bg-overlay-light hover:text-text-primary"
				>
					Custom...
				</button>
			{:else}
				<div class="flex flex-col gap-2 p-2">
					<div class="flex items-center gap-2">
						<span class="text-[11px] text-text-muted">Every</span>
						<input
							type="number"
							min="1"
							max="99"
							bind:value={interval}
							class="w-14 rounded border border-border bg-bg-tertiary px-1.5 py-0.5 text-[12px] text-text-primary outline-none focus:border-accent"
						/>
						<select
							bind:value={freq}
							class="rounded border border-border bg-bg-tertiary px-1.5 py-0.5 text-[12px] text-text-primary outline-none focus:border-accent"
						>
							<option value="DAILY">day(s)</option>
							<option value="WEEKLY">week(s)</option>
							<option value="MONTHLY">month(s)</option>
							<option value="YEARLY">year(s)</option>
						</select>
					</div>
					{#if freq === "WEEKLY"}
						<div class="flex gap-1">
							{#each WEEKDAY_LABELS as day, i}
								<button
									onclick={() => toggleDay(day)}
									class="flex h-6 w-6 items-center justify-center rounded text-[10px] font-medium transition-colors
										{byDay.includes(day)
										? 'bg-accent text-white'
										: 'bg-bg-tertiary text-text-secondary hover:bg-overlay-light'}"
								>
									{WEEKDAY_NAMES[i].charAt(0)}
								</button>
							{/each}
						</div>
					{/if}
					<div class="flex justify-end gap-1.5">
						<button
							onclick={() => (customMode = false)}
							class="rounded px-2 py-0.5 text-[11px] text-text-muted hover:text-text-secondary"
						>
							Cancel
						</button>
						<button
							onclick={applyCustom}
							class="rounded bg-accent px-2 py-0.5 text-[11px] text-white hover:bg-accent/90"
						>
							Apply
						</button>
					</div>
				</div>
			{/if}
		</Popover.Content>
	</Popover.Portal>
</Popover.Root>
