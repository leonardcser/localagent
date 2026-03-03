import type { Block, Task } from "./api";

export interface CalendarEvent {
  id: string;
  title: string;
  startMs: number;
  endMs: number;
  color: string;
  taskId: string;
  blockId?: string;
  note?: string;
  isAllDay: boolean;
  draggable: boolean;
}

export type EventWithOverlap = CalendarEvent & {
  overlapIndex: number;
  overlapCount: number;
};

export function blockToEvent(block: Block, task?: Task): CalendarEvent {
  return {
    id: `block:${block.id}`,
    title: task?.title ?? "Untitled",
    startMs: block.startAtMs,
    endMs: block.endAtMs,
    color: "var(--color-accent)",
    taskId: block.taskId,
    blockId: block.id,
    note: block.note,
    isAllDay: false,
    draggable: true,
  };
}

export function taskToEvent(task: Task): CalendarEvent | null {
  if (!task.due || task.status === "done") return null;
  const dueDate = new Date(task.due + "T00:00:00");
  return {
    id: `task:${task.id}`,
    title: task.title,
    startMs: dueDate.getTime(),
    endMs: dueDate.getTime() + 86400000,
    color:
      task.priority === "high"
        ? "var(--color-error)"
        : "var(--color-text-muted)",
    taskId: task.id,
    isAllDay: true,
    draggable: false,
  };
}

export function computeCalendarEventsOverlaps(
  events: CalendarEvent[],
): EventWithOverlap[] {
  const eventsByDay: Map<number, CalendarEvent[]> = new Map();

  for (const event of events) {
    const day = new Date(event.startMs).getDay();
    const list = eventsByDay.get(day) ?? [];
    list.push(event);
    eventsByDay.set(day, list);
  }

  const result: EventWithOverlap[] = [];

  for (const [, dayEvents] of eventsByDay) {
    dayEvents.sort((a, b) => a.startMs - b.startMs);

    const overlaps: EventWithOverlap[] = [];

    for (const event of dayEvents) {
      let overlapIndex = 0;

      while (
        overlaps.some(
          (o) => o.endMs > event.startMs && o.overlapIndex === overlapIndex,
        )
      ) {
        overlapIndex++;
      }

      const newEvent: EventWithOverlap = {
        ...event,
        overlapIndex,
        overlapCount: overlapIndex + 1,
      };

      for (const o of overlaps) {
        if (o.endMs > event.startMs) {
          o.overlapCount = Math.max(o.overlapCount, overlapIndex + 1);
          newEvent.overlapCount = Math.max(
            newEvent.overlapCount,
            o.overlapCount,
          );
        }
      }

      overlaps.push(newEvent);
      result.push(newEvent);
    }
  }

  return result;
}

export function calculateNewEventTime(
  event: EventWithOverlap,
  delta: { x: number; y: number },
  calendarWidth: number,
  rowHeight: number,
): { startMs: number; endMs: number } {
  const start = new Date(event.startMs);
  const weekday = start.getDay();
  const duration = event.endMs - event.startMs;

  const absoluteX = delta.x + (weekday * calendarWidth) / 7;
  const startHour = start.getHours();
  const startMinute = start.getMinutes();
  const absoluteY =
    delta.y + startHour * rowHeight + (startMinute / 60) * rowHeight;

  const newWeekday = Math.round((absoluteX * 7) / calendarWidth);
  const newHours = Math.floor(absoluteY / rowHeight);
  const newMinutes = Math.round(((absoluteY % rowHeight) / rowHeight) * 60);

  const newStart = new Date(start);
  const dayDiff = newWeekday - weekday;
  newStart.setDate(newStart.getDate() + dayDiff);
  newStart.setHours(newHours, newMinutes, 0, 0);

  return { startMs: newStart.getTime(), endMs: newStart.getTime() + duration };
}

export function getWeekStart(date: Date): Date {
  const d = new Date(date);
  const day = d.getDay();
  const diff = day === 0 ? -6 : 1 - day;
  d.setDate(d.getDate() + diff);
  d.setHours(0, 0, 0, 0);
  return d;
}

export function addDays(date: Date, days: number): Date {
  const d = new Date(date);
  d.setDate(d.getDate() + days);
  return d;
}

export function isSameDay(a: Date, b: Date): boolean {
  return (
    a.getFullYear() === b.getFullYear() &&
    a.getMonth() === b.getMonth() &&
    a.getDate() === b.getDate()
  );
}

const WEEKDAY_SHORT = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

export function weekdayShort(index: number): string {
  return WEEKDAY_SHORT[index] ?? "";
}

/** Convert JS getDay() (0=Sun) to our index (0=Mon) */
export function dayToIndex(jsDay: number): number {
  return jsDay === 0 ? 6 : jsDay - 1;
}
