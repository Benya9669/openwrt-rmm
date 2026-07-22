# OpenWrt RMM licensing

Copyright (c) 2026 Benya.

This repository uses separate licenses for the cloud application and the OpenWrt
components:

- `server/` and `web/` are licensed under the GNU Affero General Public License,
  version 3 only (`AGPL-3.0-only`). The canonical license text is in
  `LICENSES/AGPL-3.0-only.txt`.
- `agent/`, including the Go agent, shell compatibility agent, LuCI application,
  OpenWrt package recipes and their configuration files, is licensed under the MIT
  License. The license text is in `LICENSES/MIT.txt` and `agent/LICENSE`.
- Documentation outside those directories is licensed under `AGPL-3.0-only`, unless a
  file states otherwise.

Third-party dependencies retain their own licenses. Partner names, logos and other
third-party brand assets under `web/assets/partners/` are not licensed as part of the
software. See `NOTICE.md`.

Commercial licensing may be available from the copyright holder for organizations that
cannot use the server and web application under AGPL-3.0-only.

SPDX-License-Identifier: AGPL-3.0-only AND MIT
