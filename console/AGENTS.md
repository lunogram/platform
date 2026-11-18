# Console

These instructions apply under `console/` in addition to the repository root instructions.

## Structure

- Routes are defined in `console/src/views/router.tsx`, with pages under `console/src/views/`. Shared components are in `console/src/components/`; design-system primitives are in `console/src/components/ui/`.
- [`console/DESIGN.md`](DESIGN.md) defines the tokens, typography, spacing, and component rules. `console/stories/` shows them applied.

## Console UI

- Treat `console/stories/` as the visual design notebook. Before building or changing console UI, inspect the relevant component and page stories alongside the existing production components and `console/DESIGN.md`; match their spacing, typography, colors, density, and state patterns rather than inventing a parallel style.
- Search `console/src/components/ui/` and `console/stories/` before adding UI. Reuse or extend an existing component when its responsibility remains coherent.
- Create a new reusable component only when extension would mix responsibilities or produce an awkward API. Add or update a Storybook story for every new reusable component and every materially new reusable visual state.
- For a new page or a substantial page redesign, add or update a full-page Storybook composition showing representative populated content and important empty/error states where applicable. Use the actual UI components and deterministic fixture data from `console/stories/fixtures/`; avoid live API dependencies. Page stories are visual references, not substitutes for testing the real route. Keep stories in sync when the corresponding production UI changes.
- Stories render inside the providers in `console/.storybook/preview.tsx`: the English locale, a memory router, and fixed preferences (UTC). Add a provider there only when a reusable component cannot render without it.

## Conventions

- Use react-hook-form with zod schemas from `console/src/validation/` for forms.
- Translate user-facing text with `react-i18next`; add new keys to `console/public/locales/en.json`.
- Call the management API through the typed `oapiClient` in `console/src/oapi/client.ts`. Do not add endpoints to the legacy `console/src/api.ts`.
- Import types with `import type`; ESLint enforces it.

## Generated artifacts

- `console/src/oapi/management.generated.ts` is generated from `internal/http/controllers/v1/management/oapi/resources.yml` by `pnpm generate` (part of `make generate`). Do not edit it directly.

Run pnpm commands from `console/`:

```shell
pnpm lint
pnpm build
pnpm storybook
pnpm storybook:build
```
