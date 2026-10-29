import type { ReactNode } from "react";
import styles from "./PhotoFrame.module.css";

interface PhotoFrameProps {
  src?: string;
  alt: string;
  caption?: string;
  dimmed?: boolean;
  empty?: ReactNode;
}

export function PhotoFrame({ src, alt, caption, dimmed, empty }: PhotoFrameProps) {
  return (
    <figure className={styles.figure}>
      {src ? (
        <img className={dimmed ? `${styles.frame} ${styles.dimmed}` : styles.frame} src={src} alt={alt} />
      ) : (
        <div className={`${styles.frame} ${styles.empty}`}>{empty}</div>
      )}
      {caption && <figcaption className={styles.caption}>{caption}</figcaption>}
    </figure>
  );
}
