<h1 align="center" id="title">SDash</h1>

<p id="description">A very simple dashboard</p>

<h2>🧐 Features</h2>

Here're some of the project's best features:

*   Ability to display cards of websites
*   Single yaml file to configure
*   fuzzy search

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
