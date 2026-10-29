import { lazy, Suspense } from "react";
import type { LivenessError } from "./LivenessView";
import { Screen } from "../components/Screen";
import styles from "./LiveCheck.module.css";

// Amplify is most of the bundle; load it only on this step.
const LivenessView = lazy(() => import("./LivenessView"));

interface IntroProps {
  attempt: number;
  maxAttempts: number;
  starting: boolean;
  onStart: () => void;
}

export function LiveCheckIntro({ attempt, starting, onStart }: IntroProps) {
  return (
    <Screen title="Now a quick live check">
      {attempt === 0 && <p className="small">Your photo passed the checks.</p>}

      <ol className={styles.steps}>
        <li>Fit your face inside the oval.</li>
        <li>The screen flashes a few colours. Hold still until it stops.</li>
      </ol>

      <p>Turn your screen brightness up and take off sunglasses or a hat. It takes about 10 seconds.</p>

      {/* Replaces the warning on Amplify's start screen, which we skip. */}
      <p className={styles.warning}>
        The check flashes bright coloured light. If flashing light can affect you, for example with photosensitive
        epilepsy, don't do this check.
      </p>

      <div className="actions">
        <button type="button" className="button button-primary" onClick={onStart} disabled={starting}>
          {starting && <span className="spinner" aria-hidden="true" />}
          {starting ? "Starting…" : "Start live check"}
        </button>
      </div>
    </Screen>
  );
}

interface RunningProps {
  sessionId: string;
  onComplete: () => Promise<void>;
  onError: (e: LivenessError) => void;
  onCancel: () => void;
}

export function LiveCheckRunning(props: RunningProps) {
  return (
    <Screen title="Live check">
      <Suspense
        fallback={
          <p className={styles.loading}>
            <span className="spinner" aria-hidden="true" /> Getting the camera ready…
          </p>
        }
      >
        <LivenessView {...props} />
      </Suspense>
    </Screen>
  );
}

export function Checking() {
  return (
    <Screen title="Comparing your live check with your photo…">
      <p className={styles.loading}>
        <span className="spinner" aria-hidden="true" /> This takes a few seconds.
      </p>
    </Screen>
  );
}
