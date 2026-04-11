"use client";

import { useEffect, useId, useRef, useState, useCallback } from "react";

interface MermaidProps {
  chart: string;
  className?: string;
}

function useTheme() {
  const [isDark, setIsDark] = useState(false);

  useEffect(() => {
    const root = document.documentElement;

    function check() {
      setIsDark(root.classList.contains("dark"));
    }

    check();

    const observer = new MutationObserver((mutations) => {
      for (const mutation of mutations) {
        if (
          mutation.type === "attributes" &&
          mutation.attributeName === "class"
        ) {
          check();
        }
      }
    });

    observer.observe(root, { attributes: true, attributeFilter: ["class"] });

    return () => observer.disconnect();
  }, []);

  return isDark;
}

export function Mermaid({ chart, className }: MermaidProps) {
  const id = useId();
  const containerRef = useRef<HTMLDivElement>(null);
  const [svg, setSvg] = useState<string>("");
  const [error, setError] = useState<string | null>(null);
  const isDark = useTheme();
  const renderCountRef = useRef(0);

  const render = useCallback(async () => {
    const currentRender = ++renderCountRef.current;

    try {
      const mermaid = (await import("mermaid")).default;

      mermaid.initialize({
        startOnLoad: false,
        theme: isDark ? "dark" : "default",
        securityLevel: "loose",
        fontFamily:
          'ui-sans-serif, system-ui, -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, sans-serif',
        themeVariables: isDark
          ? {
              // Dark mode palette
              darkMode: true,
              background: "transparent",
              mainBkg: "#1e293b",
              nodeBorder: "#475569",
              lineColor: "#94a3b8",
              textColor: "#e2e8f0",
              primaryColor: "#1e293b",
              primaryTextColor: "#e2e8f0",
              primaryBorderColor: "#475569",
              secondaryColor: "#334155",
              secondaryTextColor: "#e2e8f0",
              secondaryBorderColor: "#475569",
              tertiaryColor: "#1e293b",
              tertiaryTextColor: "#e2e8f0",
              tertiaryBorderColor: "#475569",
              noteBkgColor: "#334155",
              noteTextColor: "#e2e8f0",
              noteBorderColor: "#475569",
              edgeLabelBackground: "#1e293b",
              clusterBkg: "#1e293b",
              clusterBorder: "#475569",
              titleColor: "#e2e8f0",
              actorTextColor: "#e2e8f0",
              actorBkg: "#1e293b",
              actorBorder: "#475569",
              actorLineColor: "#94a3b8",
              signalColor: "#e2e8f0",
              signalTextColor: "#e2e8f0",
              labelBoxBkgColor: "#1e293b",
              labelBoxBorderColor: "#475569",
              labelTextColor: "#e2e8f0",
              loopTextColor: "#e2e8f0",
              activationBorderColor: "#475569",
              activationBkgColor: "#334155",
              sequenceNumberColor: "#e2e8f0",
              sectionBkgColor: "#1e293b",
              altSectionBkgColor: "#334155",
              gridColor: "#475569",
              taskBkgColor: "#334155",
              taskTextColor: "#e2e8f0",
              taskBorderColor: "#475569",
              activeTaskBkgColor: "#475569",
              activeTaskBorderColor: "#64748b",
              doneTaskBkgColor: "#1e293b",
              doneTaskBorderColor: "#475569",
              critBkgColor: "#7f1d1d",
              critBorderColor: "#991b1b",
              transitionColor: "#94a3b8",
              labelColor: "#e2e8f0",
              stateLabelColor: "#e2e8f0",
            }
          : {
              // Light mode palette
              background: "transparent",
              mainBkg: "#f1f5f9",
              nodeBorder: "#cbd5e1",
              lineColor: "#64748b",
              textColor: "#1e293b",
              primaryColor: "#f1f5f9",
              primaryTextColor: "#1e293b",
              primaryBorderColor: "#cbd5e1",
              secondaryColor: "#e2e8f0",
              secondaryTextColor: "#1e293b",
              secondaryBorderColor: "#cbd5e1",
              tertiaryColor: "#f8fafc",
              tertiaryTextColor: "#1e293b",
              tertiaryBorderColor: "#cbd5e1",
              noteBkgColor: "#fef9c3",
              noteTextColor: "#1e293b",
              noteBorderColor: "#d4c85c",
              edgeLabelBackground: "#f8fafc",
              clusterBkg: "#f8fafc",
              clusterBorder: "#cbd5e1",
              titleColor: "#0f172a",
              actorTextColor: "#1e293b",
              actorBkg: "#f1f5f9",
              actorBorder: "#cbd5e1",
              actorLineColor: "#64748b",
              signalColor: "#1e293b",
              signalTextColor: "#1e293b",
              labelBoxBkgColor: "#f1f5f9",
              labelBoxBorderColor: "#cbd5e1",
              labelTextColor: "#1e293b",
              loopTextColor: "#1e293b",
              activationBorderColor: "#cbd5e1",
              activationBkgColor: "#e2e8f0",
              sequenceNumberColor: "#1e293b",
              sectionBkgColor: "#f1f5f9",
              altSectionBkgColor: "#e2e8f0",
              gridColor: "#cbd5e1",
              taskBkgColor: "#e2e8f0",
              taskTextColor: "#1e293b",
              taskBorderColor: "#cbd5e1",
              activeTaskBkgColor: "#cbd5e1",
              activeTaskBorderColor: "#94a3b8",
              doneTaskBkgColor: "#f1f5f9",
              doneTaskBorderColor: "#cbd5e1",
              critBkgColor: "#fef2f2",
              critBorderColor: "#fca5a5",
              transitionColor: "#64748b",
              labelColor: "#1e293b",
              stateLabelColor: "#1e293b",
            },
      });

      // mermaid requires a unique valid DOM id (no colons from useId)
      const safeId = `mermaid-${id.replace(/[^a-zA-Z0-9-_]/g, "")}-${currentRender}`;

      const { svg: renderedSvg } = await mermaid.render(safeId, chart.trim());

      if (currentRender === renderCountRef.current) {
        setSvg(renderedSvg);
        setError(null);
      }
    } catch (err) {
      if (currentRender === renderCountRef.current) {
        setError(
          err instanceof Error ? err.message : "Failed to render diagram",
        );
        setSvg("");
      }
    }
  }, [chart, id, isDark]);

  useEffect(() => {
    render();
  }, [render]);

  if (error) {
    return (
      <div className="my-4 rounded-lg border border-red-300 bg-red-50 p-4 text-sm text-red-800 dark:border-red-800 dark:bg-red-950 dark:text-red-200">
        <p className="font-semibold">Mermaid diagram error</p>
        <pre className="mt-2 whitespace-pre-wrap font-mono text-xs">
          {error}
        </pre>
      </div>
    );
  }

  if (!svg) {
    return (
      <div className="my-4 flex items-center justify-center rounded-lg border border-border bg-card p-8 text-sm text-muted-foreground">
        <svg
          className="mr-2 h-4 w-4 animate-spin"
          xmlns="http://www.w3.org/2000/svg"
          fill="none"
          viewBox="0 0 24 24"
        >
          <circle
            className="opacity-25"
            cx="12"
            cy="12"
            r="10"
            stroke="currentColor"
            strokeWidth="4"
          />
          <path
            className="opacity-75"
            fill="currentColor"
            d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"
          />
        </svg>
        Loading diagram…
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className={[
        "mermaid-diagram",
        "my-4 flex justify-center overflow-x-auto rounded-lg border border-border bg-card/50 p-6",
        "[&_svg]:max-w-full [&_svg]:h-auto",
        className,
      ]
        .filter(Boolean)
        .join(" ")}
      dangerouslySetInnerHTML={{ __html: svg }}
    />
  );
}
