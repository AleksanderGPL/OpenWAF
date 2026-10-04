# Third-party notices

## OWASP CRS and Coraza

OpenWAF embeds OWASP CRS 4.25.0 through `github.com/corazawaf/coraza-coreruleset/v4` and uses Coraza 3.8.1. Both projects use Apache License 2.0. Upstream rule files and their copyright notices remain unmodified in the dependency. Their licenses are reproduced in `licenses/`.

Sources:
- https://github.com/coreruleset/coreruleset
- https://github.com/corazawaf/coraza
- https://github.com/corazawaf/coraza-coreruleset

## CrowdSec rule adaptations

The environment-file and PHPUnit checks in `internal/rules/filters/exposure.conf` adapt the following CrowdSec Hub rules, retrieved 2026-10-03:

- https://github.com/crowdsecurity/hub/blob/master/appsec-rules/crowdsecurity/vpatch-env-access.yaml
- https://github.com/crowdsecurity/hub/blob/master/appsec-rules/crowdsecurity/vpatch-CVE-2017-9841.yaml

Changes: YAML conditions translated into SecLang; matching uses a case-insensitive decoded request path; environment suffix variants are also covered. The Git exposure rule is a local OpenWAF rule. No broader CrowdSec engine, scenarios, blocklists or YAML loader is included.

MIT License

Copyright (c) 2025 CrowdSecurity

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

## DB-IP Country Lite

Runtime downloads use the unmodified DB-IP Country Lite database from
https://db-ip.com/db/download/ip-to-country-lite, licensed under Creative Commons
Attribution 4.0 International: https://creativecommons.org/licenses/by/4.0/.
IP Geolocation by DB-IP: https://db-ip.com.
