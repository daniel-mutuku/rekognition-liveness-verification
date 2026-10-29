import { useCallback, useEffect, useReducer, useRef } from "react";
import { api, ApiError, isValidId } from "./api";
import { errors, type FlowError } from "./copy";
import { announcement, initialState, reducer, stepOf, type Photo, type State } from "./flow";
import { preparePhoto, UnreadableImageError } from "./lib/preparePhoto";
import { Welcome } from "./screens/Welcome";
import { PhotoStep } from "./screens/PhotoStep";
import { Checking, LiveCheckIntro, LiveCheckRunning } from "./screens/LiveCheck";
import { Result } from "./screens/Result";
import { ErrorScreen } from "./screens/ErrorScreen";
import type { LivenessError } from "./screens/LivenessView";

const MAX_ATTEMPTS = 3;

const idFromUrl = () => new URLSearchParams(location.search).get("id");
const setUrlId = (id: string | null) =>
  history.replaceState(null, "", id ? `?id=${encodeURIComponent(id)}` : location.pathname);

export function App() {
  const startId = idFromUrl();
  const [state, dispatch] = useReducer(reducer, startId !== null && isValidId(startId), initialState);

  // Only retry() reads this. The ref lags a dispatch by one render, so other
  // handlers take their inputs as arguments.
  const stateRef = useRef<State>(state);
  stateRef.current = state;

  const fail = useCallback((error: FlowError) => dispatch({ type: "failed", error }), []);

  useEffect(() => {
    if (state.kind !== "loading" || !startId) return;
    let cancelled = false;
    api.get(startId).then(
      (v) => !cancelled && dispatch({ type: "resumed", verification: v }),
      (e) => {
        if (cancelled) return;
        setUrlId(null);
        fail(apiFailure(e, "restart"));
      },
    );
    return () => {
      cancelled = true;
    };
  }, []);

  const photoUrl = useRef<string | null>(null);
  const keepPhoto = (photo: Photo | null) => {
    if (photoUrl.current && photoUrl.current !== photo?.url) URL.revokeObjectURL(photoUrl.current);
    photoUrl.current = photo?.url ?? null;
  };

  async function pickFile(file: File) {
    dispatch({ type: "preparing" });
    try {
      const blob = await preparePhoto(file);
      const photo = { blob, url: URL.createObjectURL(blob) };
      keepPhoto(photo);
      dispatch({ type: "photoPicked", photo });
    } catch (e) {
      if (e instanceof UnreadableImageError) dispatch({ type: "photoRejected", reasons: ["unreadable"] });
      else fail({ ...errors.unexpected });
    }
  }

  async function upload(photo: Photo) {
    dispatch({ type: "uploadStarted" });
    try {
      // One verification per upload; rejected ones just expire.
      const created = await api.create();
      await api.upload(created, photo.blob);
      await api.checkPhoto(created.id);
      setUrlId(created.id);
      dispatch({ type: "photoAccepted", id: created.id });
    } catch (e) {
      if (e instanceof ApiError && e.code === "photo_rejected") {
        dispatch({ type: "photoRejected", reasons: e.reasons });
      } else if (e instanceof ApiError && (e.code === "upload_failed" || e.code === "photo_missing")) {
        fail({ ...errors.uploadFailed });
      } else {
        fail(apiFailure(e, "upload"));
      }
    }
  }

  async function startLiveness(id: string) {
    dispatch({ type: "sessionRequested" });
    try {
      // Sessions expire 3 minutes after creation, so create one only on the press.
      const session = await api.startLiveness(id);
      dispatch({ type: "sessionReady", sessionId: session.sessionId, attempt: session.attempt });
    } catch (e) {
      fail(apiFailure(e, "liveness"));
    }
  }

  async function livenessComplete(id: string) {
    dispatch({ type: "livenessFinished" });
    try {
      dispatch({ type: "decided", verification: await completeWhenReady(id) });
    } catch (e) {
      fail(apiFailure(e, "liveness"));
    }
  }

  function livenessError(e: LivenessError) {
    console.warn("liveness error", e.state, e.error);
    fail(livenessFailure(e));
  }

  function retry() {
    const s = stateRef.current;
    if (s.kind !== "error") return;
    switch (s.error.retry) {
      case "upload":
        dispatch({ type: "retryPhoto" });
        if (s.photo) upload(s.photo);
        break;
      case "choosePhoto":
        setUrlId(null);
        dispatch({ type: "retryPhoto" });
        break;
      case "liveness":
        dispatch({ type: "retryLiveness" });
        if (s.id) startLiveness(s.id);
        break;
      case "restart":
        restart();
    }
  }

  function restart() {
    keepPhoto(null);
    setUrlId(null);
    dispatch({ type: "restart" });
  }

  const step = stepOf(state);

  return (
    <div className="page">
      <div className="panel">
        <header className="panel-header">
          <span className="wordmark">Photo check</span>
          {step > 0 && <span className="small">Step {step} of 3</span>}
        </header>

        <main className="panel-body">
          {state.kind === "loading" && <p className="small">Loading your verification…</p>}

          {state.kind === "welcome" && <Welcome onStart={() => dispatch({ type: "start" })} />}

          {(state.kind === "photo" || state.kind === "uploading") && (
            <PhotoStep
              photo={state.photo}
              rejected={state.kind === "photo" ? state.rejected : undefined}
              preparing={state.kind === "photo" && !!state.preparing}
              uploading={state.kind === "uploading"}
              onFile={pickFile}
              onUpload={() => state.photo && upload(state.photo)}
            />
          )}

          {state.kind === "photoChecked" && (
            <LiveCheckIntro
              attempt={state.attempt}
              maxAttempts={MAX_ATTEMPTS}
              starting={state.starting}
              onStart={() => startLiveness(state.id)}
            />
          )}

          {state.kind === "liveness" && (
            <LiveCheckRunning
              key={state.sessionId}
              sessionId={state.sessionId}
              onComplete={() => livenessComplete(state.id)}
              onError={livenessError}
              onCancel={() => dispatch({ type: "livenessCancelled" })}
            />
          )}

          {state.kind === "checking" && <Checking />}

          {state.kind === "result" && <Result verification={state.verification} onRestart={restart} />}

          {state.kind === "error" && <ErrorScreen error={state.error} onRetry={retry} onRestart={restart} />}
        </main>
      </div>

      <div className="visually-hidden" role="status" aria-live="polite">
        {announcement(state)}
      </div>
    </div>
  );
}

// The session result can lag the end of the stream by a moment.
async function completeWhenReady(id: string) {
  for (let attempt = 1; ; attempt++) {
    try {
      return await api.complete(id);
    } catch (e) {
      if (!(e instanceof ApiError && e.code === "liveness_not_finished") || attempt >= 6) throw e;
      await new Promise((r) => setTimeout(r, 750));
    }
  }
}

function apiFailure(e: unknown, here: FlowError["retry"]): FlowError {
  if (!(e instanceof ApiError)) return { ...errors.unexpected };
  switch (e.code) {
    case "network":
      return { ...errors.network, retry: here };
    case "expired":
      return { ...errors.expired };
    case "not_found":
    case "invalid_id":
      return { ...errors.notFound };
    case "too_many_attempts":
      return { ...errors.tooManyAttempts };
    case "liveness_not_finished":
      return { ...errors.livenessStopped };
    default:
      return { ...errors.unexpected };
  }
}

function livenessFailure({ state, error }: LivenessError): FlowError {
  switch (state) {
    case "CAMERA_ACCESS_ERROR":
      // Both "denied" and "no camera" arrive as CAMERA_ACCESS_ERROR.
      return error?.name === "NotFoundError" || error?.name === "OverconstrainedError"
        ? { ...errors.noCamera }
        : { ...errors.cameraBlocked };
    case "DEFAULT_CAMERA_NOT_FOUND_ERROR":
      return { ...errors.noCamera };
    case "TIMEOUT":
    case "CONNECTION_TIMEOUT":
    case "FRESHNESS_TIMEOUT":
    case "SERVER_ERROR": // includes an expired session
      return { ...errors.timedOut };
    case "MULTIPLE_FACES_ERROR":
      return { ...errors.multipleFaces };
    case "FACE_DISTANCE_ERROR":
      return { ...errors.tooClose };
    case "MOBILE_LANDSCAPE_ERROR":
      return { ...errors.landscape };
    case "CAMERA_FRAMERATE_ERROR":
      return { ...errors.slowCamera };
    default:
      return { ...errors.livenessStopped };
  }
}
