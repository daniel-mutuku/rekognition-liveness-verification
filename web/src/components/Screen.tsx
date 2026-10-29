import { useEffect, useRef, type ReactNode } from "react";

// After the first screen, focus moves to each new heading so keyboard and
// screen reader users aren't left on a button that no longer exists.
let firstScreen = true;

interface ScreenProps {
  title: ReactNode;
  headingClassName?: string;
  before?: ReactNode;
  children: ReactNode;
}

export function Screen({ title, headingClassName, before, children }: ScreenProps) {
  const heading = useRef<HTMLHeadingElement>(null);

  useEffect(() => {
    if (firstScreen) {
      firstScreen = false;
      return;
    }
    heading.current?.focus();
  }, []);

  return (
    <section className="stack-lg">
      {before}
      <h1 ref={heading} tabIndex={-1} className={headingClassName}>
        {title}
      </h1>
      {children}
    </section>
  );
}
