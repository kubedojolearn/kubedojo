/**
 * Lab panels run commands in more than one place (the terminal, the editor's Apply button). Each
 * result is published here so the terminal shows every command the lab ran, in order.
 */

export interface LabCommandEvent {
  labId: string;
  requestId: string;
  input: string;
  stdout: string;
  stderr: string;
  exitCode: number;
}

const EVENT = "kubedojo:lab-command";

export function publishCommand(e: LabCommandEvent) {
  window.dispatchEvent(new CustomEvent<LabCommandEvent>(EVENT, { detail: e }));
}

/** Subscribes to commands run outside the terminal; returns the unsubscribe function. */
export function onCommand(labId: string, fn: (e: LabCommandEvent) => void): () => void {
  const listener = (ev: Event) => {
    const detail = (ev as CustomEvent<LabCommandEvent>).detail;
    if (detail.labId === labId) fn(detail);
  };
  window.addEventListener(EVENT, listener);
  return () => window.removeEventListener(EVENT, listener);
}
