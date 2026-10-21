export interface Program {
  ID: number;
  Name: string;
  Description: string;
  StartAt: string;
  EndAt: string;
}
export interface Service {
  ID: number;
  Name: string;
  Type: string;
  Number: number;
  Current: Program | null;
  Programs: Program[] | null;
}
export interface Snapshot {
  Occupied: number;
  Limit: number;
  RecordingServices: number[] | null;
}
export interface PageMeta {
  Title: string;
  Active: string;
  Message: string;
  Error: string;
  Recording: Snapshot;
}
export interface Capabilities {
  Available: boolean;
  Warning: string;
  Selected: { Name: string; Hardware: boolean };
}
export interface LiveStatus {
  ServiceID: number;
  Encoder: string;
  Hardware: boolean;
  Ready: boolean;
  Warning: string;
  Error: string;
}
export interface HomeData {
  Page: PageMeta;
  Services: Service[] | null;
  Capabilities: Capabilities;
  BackendError: string;
}
export interface LiveData {
  Page: PageMeta;
  Service: Service;
  Services: Service[] | null;
  Live: LiveStatus;
  BackendError: string;
}
export interface Reservation {
  id: string;
  title: string;
  startAt: string;
  endAt: string;
  state: "scheduled" | "recording" | "completed" | "failed" | "cancelled";
  error?: string;
}
export interface LibraryData {
  Page: PageMeta;
  Reservations: Reservation[] | null;
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const response = await fetch(path, { cache: "no-store", ...init });
  const json = response.headers
    .get("Content-Type")
    ?.includes("application/json");
  const data = json ? await response.json() : null;
  if (!response.ok)
    throw new Error(
      data?.error || `Request failed (${response.status}). Please try again.`,
    );
  if (!json)
    throw new Error(
      "The server returned an unexpected response. Please refresh and try again.",
    );
  return data as T;
}

export function mutate(path: string, programID?: number) {
  return request<{ message: string; error: string }>(path, {
    method: "POST",
    body: new URLSearchParams(
      programID === undefined ? {} : { program_id: String(programID) },
    ),
  });
}

export const clock = (value: string) =>
  new Date(value).toLocaleTimeString(undefined, {
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
export const dateTime = (value: string) =>
  new Date(value).toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  });
export function duration(start: string, end: string) {
  const minutes = Math.round((Date.parse(end) - Date.parse(start)) / 60000);
  return minutes < 60
    ? `${minutes} min`
    : `${Math.floor(minutes / 60)}h ${String(minutes % 60).padStart(2, "0")}m`;
}
