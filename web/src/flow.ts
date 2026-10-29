// welcome → photo → uploading → photoChecked → liveness → checking → result,
// plus error from any step. API calls live in App; this only decides transitions.
import type { Reason, Verification } from "./api";
import type { FlowError } from "./copy";

export interface Photo {
  blob: Blob;
  url: string;
}

export type State =
  | { kind: "loading" }
  | { kind: "welcome" }
  | { kind: "photo"; photo?: Photo; rejected?: Reason[]; preparing?: boolean }
  | { kind: "uploading"; photo: Photo }
  | { kind: "photoChecked"; id: string; photoUrl: string; attempt: number; starting: boolean }
  | { kind: "liveness"; id: string; photoUrl: string; sessionId: string; attempt: number }
  | { kind: "checking"; id: string; photoUrl: string }
  | { kind: "result"; verification: Verification }
  | {
      kind: "error";
      error: FlowError;
      // Context for retrying from where it failed.
      id?: string;
      photo?: Photo;
      photoUrl?: string;
      attempt?: number;
    };

export type Event =
  | { type: "start" }
  | { type: "preparing" }
  | { type: "photoPicked"; photo: Photo }
  | { type: "uploadStarted" }
  | { type: "photoAccepted"; id: string }
  | { type: "photoRejected"; reasons: Reason[] }
  | { type: "sessionRequested" }
  | { type: "sessionReady"; sessionId: string; attempt: number }
  | { type: "livenessCancelled" }
  | { type: "livenessFinished" }
  | { type: "decided"; verification: Verification }
  | { type: "resumed"; verification: Verification }
  | { type: "failed"; error: FlowError }
  | { type: "retryLiveness" }
  | { type: "retryPhoto" }
  | { type: "restart" };

export const initialState = (hasId: boolean): State => (hasId ? { kind: "loading" } : { kind: "welcome" });

export function reducer(state: State, event: Event): State {
  switch (event.type) {
    case "start":
      return state.kind === "welcome" ? { kind: "photo" } : state;

    case "preparing":
      return state.kind === "photo" ? { kind: "photo", preparing: true } : state;

    case "photoPicked":
      return state.kind === "photo" ? { kind: "photo", photo: event.photo } : state;

    case "uploadStarted":
      return state.kind === "photo" && state.photo ? { kind: "uploading", photo: state.photo } : state;

    case "photoAccepted":
      return state.kind === "uploading"
        ? { kind: "photoChecked", id: event.id, photoUrl: state.photo.url, attempt: 0, starting: false }
        : state;

    case "photoRejected":
      if (state.kind === "uploading") return { kind: "photo", photo: state.photo, rejected: event.reasons };
      // The browser couldn't decode it, so there's no preview.
      if (state.kind === "photo") return { kind: "photo", rejected: event.reasons };
      return state;

    case "sessionRequested":
      return state.kind === "photoChecked" ? { ...state, starting: true } : state;

    case "sessionReady":
      return state.kind === "photoChecked"
        ? { kind: "liveness", id: state.id, photoUrl: state.photoUrl, sessionId: event.sessionId, attempt: event.attempt }
        : state;

    case "livenessCancelled":
      return state.kind === "liveness"
        ? { kind: "photoChecked", id: state.id, photoUrl: state.photoUrl, attempt: state.attempt, starting: false }
        : state;

    case "livenessFinished":
      return state.kind === "liveness" ? { kind: "checking", id: state.id, photoUrl: state.photoUrl } : state;

    case "decided":
      return state.kind === "checking" ? { kind: "result", verification: event.verification } : state;

    case "resumed": {
      if (state.kind !== "loading") return state;
      const v = event.verification;
      if (v.status === "approved" || v.status === "review" || v.status === "rejected") {
        return { kind: "result", verification: v };
      }
      if ((v.status === "photo_ok" || v.status === "liveness_started") && v.photoUrl) {
        return { kind: "photoChecked", id: v.id, photoUrl: v.photoUrl, attempt: v.livenessAttempts, starting: false };
      }
      return { kind: "welcome" };
    }

    case "failed":
      return { kind: "error", error: event.error, ...context(state) };

    case "retryLiveness":
      return state.kind === "error" && state.id && state.photoUrl
        ? { kind: "photoChecked", id: state.id, photoUrl: state.photoUrl, attempt: state.attempt ?? 0, starting: false }
        : state;

    case "retryPhoto":
      return state.kind === "error" ? { kind: "photo", photo: state.photo } : state;

    case "restart":
      return { kind: "welcome" };
  }
}

function context(state: State): Omit<Extract<State, { kind: "error" }>, "kind" | "error"> {
  switch (state.kind) {
    case "photo":
    case "uploading":
      return { photo: state.photo };
    case "photoChecked":
    case "liveness":
      return { id: state.id, photoUrl: state.photoUrl, attempt: state.attempt };
    case "checking":
      return { id: state.id, photoUrl: state.photoUrl };
    case "error":
      return { id: state.id, photo: state.photo, photoUrl: state.photoUrl, attempt: state.attempt };
    default:
      return {};
  }
}

export function stepOf(state: State): 0 | 1 | 2 | 3 {
  switch (state.kind) {
    case "photo":
    case "uploading":
      return 1;
    case "photoChecked":
    case "liveness":
    case "checking":
      return 2;
    case "result":
      return 3;
    case "error":
      return state.id ? 2 : 1;
    default:
      return 0;
  }
}

export function announcement(state: State): string {
  switch (state.kind) {
    case "photo":
      if (state.rejected) return "This photo won't work. The reasons are listed below.";
      if (state.preparing) return "Preparing your photo";
      return state.photo ? "Photo selected. Upload it to check it." : "";
    case "uploading":
      return "Checking your photo";
    case "photoChecked":
      return state.starting ? "Starting the live check" : "Your photo looks good. Next, the live check.";
    case "liveness":
      return "Live check started";
    case "checking":
      return "Comparing your live check with your photo";
    case "result":
      return verdict(state.verification.status);
    case "error":
      return `${state.error.title}. ${state.error.body}`;
    default:
      return "";
  }
}

export function verdict(status: Verification["status"]): string {
  switch (status) {
    case "approved":
      return "You're verified";
    case "review":
      return "We need to check this by hand";
    default:
      return "We couldn't confirm it's you";
  }
}
