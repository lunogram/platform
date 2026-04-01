import type { ReactNode } from "react";

interface EnterpriseFeatureProps {
  children?: ReactNode;
}

export function EnterpriseFeature({ children }: EnterpriseFeatureProps) {
  return (
    <div className="not-prose my-6 rounded-lg border border-amber-200 bg-amber-50 p-4 dark:border-amber-500/20 dark:bg-amber-500/5">
      <div className="flex items-center gap-2">
        <span className="inline-flex items-center rounded-md bg-amber-100 px-2.5 py-0.5 text-xs font-semibold text-amber-800 dark:bg-amber-500/15 dark:text-amber-300">
          Enterprise
        </span>
        <span className="text-sm text-amber-700 dark:text-amber-300">
          This feature is available in Enterprise and Cloud.
        </span>
      </div>
      {children && (
        <div className="mt-2 text-sm text-amber-600 dark:text-amber-400">
          {children}
        </div>
      )}
    </div>
  );
}

export function EnterpriseBadge() {
  return (
    <span className="not-prose ml-2 inline-flex items-center rounded-md bg-amber-100 px-2 py-0.5 align-middle text-xs font-semibold text-amber-800 dark:bg-amber-500/15 dark:text-amber-300">
      Enterprise
    </span>
  );
}
