import { useEffect, useRef } from "react";

import { runEventsURL, type RunEvent } from "../api/client";

interface RunEventHandlers {
  onDelta(delta: string): void;
  onTerminal(event: RunEvent): void;
  onConnectionChange(connected: boolean): void;
}

export function useRunEvents(
  runId: string | undefined,
  handlers: RunEventHandlers,
): void {
  const handlersRef = useRef(handlers);
  handlersRef.current = handlers;

  useEffect(() => {
    if (!runId) return;
    const source = new EventSource(runEventsURL(runId));
    const eventTypes = [
      "message.delta",
      "message.completed",
      "run.failed",
      "run.cancelled",
    ];
    source.onopen = () => handlersRef.current.onConnectionChange(true);
    source.onerror = () => handlersRef.current.onConnectionChange(false);
    for (const type of eventTypes) {
      source.addEventListener(type, (raw) => {
        const event = JSON.parse(
          (raw as MessageEvent<string>).data,
        ) as RunEvent;
        if (type === "message.delta") {
          const payload = event.payload as { delta?: string };
          if (payload.delta) handlersRef.current.onDelta(payload.delta);
          return;
        }
        handlersRef.current.onTerminal(event);
        source.close();
      });
    }
    return () => source.close();
  }, [runId]);
}
