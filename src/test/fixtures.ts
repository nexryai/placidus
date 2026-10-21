import type { HomeData, LibraryData, LiveData } from "../api";

const recording = { Occupied: 0, Limit: 1, RecordingServices: [] };
const program = {
  ID: 99,
  Name: "Morning News",
  Description: "The latest news.",
  StartAt: "2026-10-21T00:00:00Z",
  EndAt: "2026-10-21T01:00:00Z",
};
const service = {
  ID: 100,
  Name: "Test TV",
  Type: "GR",
  Number: 1,
  Current: program,
  Programs: [program],
};
export const home: HomeData = {
  Page: {
    Title: "Home",
    Active: "home",
    Message: "",
    Error: "",
    Recording: recording,
  },
  Services: [service],
  BackendError: "",
  Capabilities: {
    Available: true,
    Selected: { Name: "h264_nvenc", Hardware: true },
    Warning: "",
  },
};
export const live: LiveData = {
  Page: { ...home.Page, Title: "Test TV", Active: "live" },
  Services: [service],
  Service: service,
  BackendError: "",
  Live: {
    ServiceID: 100,
    Encoder: "h264_nvenc",
    Hardware: true,
    Ready: true,
    Warning: "",
    Error: "",
  },
};
export const library: LibraryData = {
  Page: { ...home.Page, Title: "Reservations", Active: "reservations" },
  Reservations: [
    {
      id: "r1",
      title: program.Name,
      startAt: program.StartAt,
      endAt: program.EndAt,
      state: "scheduled",
    },
  ],
};
