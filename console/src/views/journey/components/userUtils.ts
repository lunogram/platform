/**
 * Re-export the centralized user display-name utilities.
 *
 * All name-resolution logic lives in `@/lib/name` — this module exists only to
 * keep existing imports working without a large-scale search-and-replace.
 */
export { getUserDisplayName, getUserInitials, getUserSubtext } from "@/lib/name"
