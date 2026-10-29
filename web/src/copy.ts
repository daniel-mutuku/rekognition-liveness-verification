import type { Reason } from "./api";

type PhotoReason =
  | "no_face"
  | "multiple_faces"
  | "blurry"
  | "too_dark"
  | "face_turned"
  | "too_large"
  | "unreadable";

const photoReasons: Record<PhotoReason, string> = {
  no_face: "We couldn't find a face in this photo. Choose one where your face is clearly visible.",
  multiple_faces: "This photo has more than one face. Choose a photo of just you.",
  blurry: "This photo is too blurry to compare. Choose a sharper one, taken in steady light.",
  too_dark: "This photo is too dark. Choose one taken in daylight or a well-lit room.",
  face_turned: "Your face is turned away from the camera. Choose a photo where you look straight ahead.",
  too_large: "This file is over 15 MB. Choose a smaller photo.",
  unreadable: "We couldn't open this file as a photo. Choose a JPEG or PNG.",
};

type ResultReason =
  | "liveness_uncertain"
  | "liveness_failed"
  | "match_uncertain"
  | "face_mismatch"
  | "liveness_session_failed"
  | "no_face_in_reference"
  | "photo_changed";

const resultReasons: Record<ResultReason, string> = {
  liveness_uncertain: "The live check wasn't sure it saw a real person. Better light usually helps.",
  liveness_failed:
    "The live check didn't detect a real person in front of the camera. Do it again without holding up a photo or screen.",
  match_uncertain: "Your face matched, but not closely enough. A clearer photo usually helps.",
  face_mismatch: "The face in the live check doesn't look like the face in your photo.",
  liveness_session_failed:
    "The live check didn't finish. Start again and keep your face in the oval until it's done.",
  no_face_in_reference: "The live check couldn't capture your face clearly. Start again in better light.",
  photo_changed: "Your photo was replaced after it was checked. Start again.",
};

function lookup(table: Record<string, string>, code: Reason, fallback: string): string {
  return table[code] ?? fallback;
}

export const photoReasonText = (code: Reason) =>
  lookup(photoReasons, code, "This photo can't be used. Choose another photo.");

export const resultReasonText = (code: Reason) =>
  lookup(resultReasons, code, "One of the checks didn't pass.");

export type RetryAction = "upload" | "choosePhoto" | "liveness" | "restart";

export const retryLabel: Record<RetryAction, string> = {
  upload: "Upload photo",
  choosePhoto: "Choose another photo",
  liveness: "Start live check",
  restart: "Start again",
};

export interface FlowError {
  title: string;
  body: string;
  retry: RetryAction;
}

export const errors = {
  network: {
    title: "We can't reach the server",
    body: "Check your connection, then try again in a moment.",
  },
  uploadFailed: {
    title: "Your photo didn't upload",
    body: "Check your connection and upload it again.",
    retry: "upload",
  },
  cameraBlocked: {
    title: "Camera access is blocked",
    body: "Allow camera access for this site in your browser's settings (the icon next to the address bar), then start the live check again.",
    retry: "liveness",
  },
  noCamera: {
    title: "No camera found",
    body: "Connect a camera, or open this page on a phone or laptop that has one.",
    retry: "liveness",
  },
  timedOut: {
    title: "The live check timed out",
    body: "It has to be finished within 3 minutes. Start it again; it takes about 10 seconds.",
    retry: "liveness",
  },
  multipleFaces: {
    title: "More than one face was in view",
    body: "Make sure only your face is in front of the camera, then start the live check again.",
    retry: "liveness",
  },
  tooClose: {
    title: "Your face was too close to the camera",
    body: "Hold the camera about an arm's length away, then start the live check again.",
    retry: "liveness",
  },
  landscape: {
    title: "Turn your phone upright",
    body: "The live check only works in portrait. Turn your phone, then start the live check again.",
    retry: "liveness",
  },
  slowCamera: {
    title: "Your camera is too slow for the check",
    body: "Close other apps that use the camera, or try another device.",
    retry: "liveness",
  },
  livenessStopped: {
    title: "The live check stopped",
    body: "Start it again. If it keeps happening, reload the page.",
    retry: "liveness",
  },
  tooManyAttempts: {
    title: "You've used all 3 live checks",
    body: "Each verification allows 3. Start again with your photo.",
    retry: "choosePhoto",
  },
  expired: {
    title: "This verification has expired",
    body: "Verifications are deleted after 7 days. Start a new one.",
    retry: "restart",
  },
  notFound: {
    title: "We couldn't find this verification",
    body: "The link may be incomplete, or the verification was deleted. Start a new one.",
    retry: "restart",
  },
  unexpected: {
    title: "Something went wrong on our side",
    body: "Nothing you did caused this. Start again, and if it keeps happening, try later.",
    retry: "restart",
  },
} satisfies Record<string, FlowError | Omit<FlowError, "retry">>;
