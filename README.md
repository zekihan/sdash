<h1 align="center" id="title">SDash</h1>

<p id="description">A very simple dashboard</p>

<h2>🧐 Features</h2>

Here're some of the project's best features:

*   Ability to display cards of websites
*   Single yaml file to configure
*   fuzzy search

<h2>Icon proxy</h2>

Set `proxyIcon: true` on a service to fetch its icon through `/api/proxy/icon`.
The proxy accepts PNG, JPEG and GIF responses with matching image content types,
and decodes and re-encodes them as PNG. GIF animation is reduced to its first frame.
SVG, HTML, other formats, redirects and non-2xx responses are rejected. If you
currently proxy SVG icons, configure a raster version before upgrading. The sample
dashboard's SVG icons load directly because `proxyIcon` defaults to `false`.

Proxied URLs must use HTTP on port 80 or HTTPS on port 443. Every DNS result must
be a public address; private, loopback, link-local, metadata and special-use
addresses are blocked. Connections use the validated IP directly while preserving
the original hostname for Host and TLS verification. Environment HTTP proxies
are bypassed so they cannot resolve or route around this check. This endpoint
is intended for public icons, and cannot fetch icons hosted on a private network.

Each upstream body and re-encoded PNG is limited to 1 MiB. Images are limited to
2,048 pixels on either axis and 1,048,576 pixels in total before decoding. The
cache retains at most 100 images and 8 MiB of image buffer capacity for ten
minutes. At most four distinct fetch/decode jobs run at once; simultaneous misses
for the same URL share a job. Overload returns 503. A bounded per-peer limiter
allows 120 requests per minute, including cache hits, and returns 429 with
`Retry-After` when exhausted. It uses the connection peer, not forwarded headers,
so callers behind a reverse proxy share that proxy's budget.

The checked-in listener binds to loopback and OIDC is disabled. Deployments that
expose SDash to other users should review their authentication and egress policy.
Local security tests use synthetic DNS answers and mocked/in-memory HTTP peers;
production networking, reverse-proxy budgets and browser acceptance require a
separately authorized deployment check.

<h2>💻 Built with</h2>

Technologies used in the project:

*   go
*   htmx
*   tailwindcss
*   templ

<h2>🛡️ License:</h2>

This project is licensed under the GNU Affero General Public License v3.0

### Authentication lifetime and access logs

`web.sessionTimeout` (or `SDS__WEB_SESSION_TIMEOUT`) sets the authenticated
session expiry and the browser cookie's `Max-Age` and `Expires`. The default is
`24h`. Set a whole number of seconds from `1s` through `720h` (30 days); invalid
values prevent startup. Existing signed sessions retain their original expiry;
the configured lifetime applies to newly issued sessions.

Access logs include method, path, status, response size and duration. They omit
all query values, full URLs, headers and bodies, including callback `code` and
`state` and proxy URLs containing credentials. OIDC failure logs identify the
failed operation without recording provider error bodies, which may echo secrets.
Operators must configure equivalent query omission/redaction in reverse proxies
and centralized log pipelines. Those deployment settings are not managed by SDash.

### Dashboard hot reload

Set `dashboard.reload.enabled` in `config.yaml` to reload `dashboard.yaml`.
With `useFsNotify: true`, SDash watches its parent directory and handles writes,
creation, removal, and renaming of the configured filename. This supports editors
that save by atomic replacement, including repeated replacements and later writes.
Changes to other filenames do not trigger reloads.

The default polling mode compares the complete modification timestamp on every
`pollInterval`, including subsecond differences. Any different timestamp reloads,
including an older timestamp on a replacement. Changes that preserve exactly the
same timestamp are not detected by polling; use fsnotify for those saves. Polling
observes the latest file at each interval, not every intermediate edit.

With `continueOnError: true`, missing or temporarily invalid dashboard files keep
the last successfully loaded dashboard; later valid changes can reload normally.
With it disabled, a reload error still panics. Polling skips missing files until
they reappear; fsnotify removal events attempt a reload immediately.
