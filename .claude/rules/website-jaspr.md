---
paths:
  - "website/**"
---
# Website (Jaspr, Section 10)
- Jaspr `^0.23.5`, server mode; nijer resolution ar `pubspec.lock` (4.6), `shared` path dependency. Code er age `website/` e `jaspr install-skills --agent claude-code`; Jaspr 1.0 er age syntax bodlay, tai purono syntax mone na rekhe skill ar https://docs.jaspr.site dekho.
- Dam, stock, discount, total kokhono hishab na; Go API ja dey ta-i dekhao (Rule 1, 10.9).
- Browser kokhono Go API te jay na: `@client` component ar form `/_/...` BFF e (10.9): Origin check, cookie theke `Authorization: Bearer` ar `X-Cart-Key`, `X-Shop-Host`, `X-Client-IP` + `X-Internal-Auth`, `Cache-Control: no-store`.
- Rong shudhu CSS variable ba Tailwind token (9.3), lekha translation key (Rule 4, 9.8): component e hex rong ba Bangla/English string na.
- `@client` shudhu dorkari jaygay (10.2); mobile-first 360px, input 16px (10.5).
- Token page (`/order/*`, `/confirm/*`, `/invoice/*`, `/print/*`): third-party script na, `noindex`, `no-referrer` (10.1, 10.4).
- Session, cart ba preview cookie thaka response e `Cache-Control: private, no-store` (10.3).
