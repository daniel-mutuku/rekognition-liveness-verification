import type { CSSProperties } from "react";
import type { Verification } from "../api";
import { resultReasonText } from "../copy";
import { verdict } from "../flow";
import { CrossIcon, HalfCircleIcon, TickIcon } from "../components/Icons";
import { PhotoFrame } from "../components/PhotoFrame";
import { Screen } from "../components/Screen";
import styles from "./Result.module.css";

const outcome = {
  approved: { Icon: TickIcon, tone: styles.moss },
  review: { Icon: HalfCircleIcon, tone: styles.ochre },
  rejected: { Icon: CrossIcon, tone: styles.brick },
} as const;

function lead(v: Verification): string {
  if (v.status === "approved") return "The live check shows a real person and it matches your photo.";
  if (v.status === "review") {
    return "The match was close but not clear enough to approve automatically. Someone will compare the photos.";
  }
  const first = v.reasons[0];
  if (first === "face_mismatch") return "The live check didn't match your photo closely enough.";
  if (first === "photo_changed") return "Your photo changed during the check.";
  return "The live check didn't succeed.";
}

export function Result({ verification: v, onRestart }: { verification: Verification; onRestart: () => void }) {
  const status = v.status === "approved" || v.status === "review" ? v.status : "rejected";
  const { Icon, tone } = outcome[status];

  return (
    <Screen
      headingClassName={styles.verdict}
      title={
        <>
          <Icon className={`${styles.icon} ${tone}`} />
          <span>{verdict(v.status)}</span>
        </>
      }
    >
      <p>{lead(v)}</p>

      <div className={styles.photos}>
        <PhotoFrame src={v.photoUrl} alt="Your profile photo" caption="Your photo" empty="No photo" />
        <PhotoFrame
          src={v.referenceUrl}
          alt="Still frame from your live check"
          caption="Live check"
          empty="No frame from the live check"
        />
      </div>

      {(v.livenessConfidence !== undefined || v.similarity !== undefined) && (
        <dl className={styles.scores}>
          {v.livenessConfidence !== undefined && (
            <Score
              label="Liveness"
              hint="Real person, not a photo or screen"
              score={v.livenessConfidence}
              passMark={v.thresholds.approveLiveness}
            />
          )}
          {v.similarity !== undefined && (
            <Score
              label="Match"
              hint="Same face as your photo"
              score={v.similarity}
              passMark={v.thresholds.approveSimilarity}
            />
          )}
        </dl>
      )}

      {v.reasons.length > 0 && (
        <div className="stack">
          <h2>Why</h2>
          <ul className={styles.reasons}>
            {v.reasons.map((code) => (
              <li key={code}>{resultReasonText(code)}</li>
            ))}
          </ul>
        </div>
      )}

      <div className="actions">
        <button
          type="button"
          className={`button ${status === "rejected" ? "button-primary" : "button-secondary"}`}
          onClick={onRestart}
        >
          Start again
        </button>
      </div>
    </Screen>
  );
}

interface ScoreProps {
  label: string;
  hint: string;
  score: number;
  passMark: number;
}

function Score({ label, hint, score, passMark }: ScoreProps) {
  const passed = score >= passMark;
  // Round down: 89.96 must not read "90.0" next to a pass mark of 90.
  const shown = (Math.floor(score * 10) / 10).toFixed(1);
  const clamp = (n: number) => Math.min(100, Math.max(0, n));

  return (
    <div className={styles.score}>
      <dt className={styles.scoreRow}>
        <span className={styles.scoreLabel}>{label}</span>
        <span className="small">{hint}</span>
      </dt>
      <dd className={styles.scoreValue}>
        <span className={styles.number}>{shown}</span>
        <span className="small">
          {passed ? "Pass mark" : "Below the pass mark of"} {passMark}
        </span>
        <span className={styles.track} aria-hidden="true">
          <span
            className={`${styles.fill} ${passed ? "" : styles.failed}`}
            style={{ "--score": clamp(score) / 100 } as CSSProperties}
          />
          <span className={styles.mark} style={{ left: `${clamp(passMark)}%` }} />
        </span>
      </dd>
    </div>
  );
}
