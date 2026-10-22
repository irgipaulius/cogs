# CoGS brand assets

| File | Use |
|------|-----|
| `logo.png` | GitHub READMEs and docs |
| `logo-animated.svg` | In-app wordmark (cloud + “CoGS” type) |
| `icon.svg` | Favicon |
| `spinner.svg` | Loading states (1.4s gear rotation) |
| `logo-static.svg` | Static wordmark export |

## Where to use

**Use CoGS brand** in framework apps shipped with this repo: **admin**, **todo**, and future first-party helpers under `apps/` that are part of the toolkit (not demo/third-party products).

**Do not use** in apps that are not the CoGS framework itself — e.g. Goldilocks, Rampocalypse, or anything installed from an external GitHub repo. Those keep their own favicon and UI identity.

React: `docs/brand/react/CogsMark.jsx` — `CogsLogo`, `CogsSpinner`, `CogsIcon`. Imported in branded apps via `@cogs/brand` (see `apps/admin/frontend`, `apps/todo/frontend`).
