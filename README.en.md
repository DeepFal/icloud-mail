<div align="center">

**[简体中文](README.md) | English**

# iCloud Mail

**A self-hosted iCloud mail management and retrieval dashboard maintained by DeepFal.**

Manage inboxes, real mailbox addresses, and local notes; track service registration states; and provide on-demand retrieval and code access to everyday tools through mailbox pools and APIs.

[Features](#features) · [Source build](#quick-start) · [Documentation](#documentation)

</div>

## Features

- Inbox and on-demand retrieval: view messages centrally, trigger incremental reads through retrieval links or APIs, and reuse retrieval connections within a primary account.
- Real mailboxes and local notes: store notes by full mailbox address, independently from Apple directory labels; notes can be restored when the same address is imported again.
- Service registry and states: record services and account identifiers for each mailbox with distinct states for unrecorded, registered, pending confirmation, confirmed unused, and disabled. This is local record-keeping and does not register third-party accounts.
- Multiple accounts: manage multiple Apple primary accounts, Hide My Email addresses, synchronization, and connection status in one dashboard.
- Mailbox pools and APIs: issue Pool Keys for projects, acquire, confirm, renew, release, and retire addresses, and retrieve verification codes through mailbox APIs.
- Batch tasks: batch synchronization, creation, deletion, registration, and confirmation; automatic creation plans can be configured per account and remain subject to upstream limits.
- Light and dark themes: choose a light or dark interface with frontend assets served locally by the service.

See [mailbox notes and service registration](docs/MAIL-REGISTRY.md) for the registry semantics and daily workflow.

## Quick Start

You need Docker Engine, Docker Compose v2, and Git. Source builds require access to Docker base images, Go modules, and npm dependencies. At least 2 CPU cores and 4 GB RAM are recommended; run a single application instance.

```sh
git clone https://github.com/DeepFal/icloud-mail.git
cd icloud-mail
cp -n .env.example .env
docker compose -f compose.yaml -f compose.build.yaml up -d --build --wait
docker compose -f compose.yaml -f compose.build.yaml ps
```

Once the target version is published to GHCR, you can use `ghcr.io/deepfal/icloud-mail` and `ghcr.io/deepfal/icloud-mail-postgres`: set `ICLOUD_MAIL_VERSION=v0.2.9` in `.env`, then run `docker compose pull` and `docker compose up -d --wait`. Both images use the same version. Use the source build above until the first image publication completes. OCI labels record the full commit, version, and source repository; see [DELIVERY.md](DELIVERY.md#ghcr-镜像) for verification.

With the example configuration, the dashboard is at `http://127.0.0.1:8788/admin/` and the username is `admin`. The generated management path and password are stored in the container's keys volume:

```sh
docker compose -f compose.yaml -f compose.build.yaml exec -T icloud-api cat /app/keys/admin-path
docker compose -f compose.yaml -f compose.build.yaml exec -T icloud-api cat /app/keys/admin-password
```

In the dashboard, follow “add a primary account → configure mail retrieval → connect to Apple → sync addresses”, then copy a retrieval link or add addresses to a mailbox pool. Direct iCloud IMAP retrieval uses an Apple app-specific password, configured separately from the account password used to manage Hide My Email.

For a remote server, use an SSH tunnel to reach the dashboard:

```sh
ssh -N -L 127.0.0.1:8788:127.0.0.1:8788 USER@SERVER
```

Back up the database, keys, and mail archive before updates; preserve `.env` and existing data volumes. See [DELIVERY.md](DELIVERY.md) and the [public deployment guide](docs/PUBLIC.md) for installation, backups, migration, and public HTTPS.

To upgrade an image installation, change `ICLOUD_MAIL_VERSION` in `.env` to the published target version, then pull and recreate both services:

```sh
# First edit .env: ICLOUD_MAIL_VERSION=vX.Y.Z (use a published version)
docker compose pull
docker compose up -d
```

Remove or update any existing `ICLOUD_API_IMAGE` / `ICLOUD_API_POSTGRES_IMAGE` overrides first. Source-build update commands are in [DELIVERY.md](DELIVERY.md#数据备份与更新).

## API Example

Use the API Key shown for a mailbox in the dashboard to retrieve verification codes:

```sh
curl -H 'Authorization: Bearer MAILBOX_API_KEY' \
  https://HOST/api/v1/otp
```

See [docs/POOL.md](docs/POOL.md), [docs/openapi.yaml](docs/openapi.yaml), and the deployed `/docs/` endpoint for mailbox-pool integration and request definitions.

## Documentation

| Topic | Documentation |
| --- | --- |
| Installation, updates, backups, and migration | [DELIVERY.md](DELIVERY.md) |
| Public access and HTTPS | [docs/PUBLIC.md](docs/PUBLIC.md) |
| Mailbox pools and programmatic access | [docs/POOL.md](docs/POOL.md) · [docs/openapi.yaml](docs/openapi.yaml) |
| Automatic creation and login recovery | [docs/AUTO_CREATION_CHANNELS.md](docs/AUTO_CREATION_CHANNELS.md) · [docs/AUTO_LOGIN.md](docs/AUTO_LOGIN.md) |
| Mail retrieval and configuration | [docs/MAIL-RECEPTION.md](docs/MAIL-RECEPTION.md) · [docs/REFERENCE.md](docs/REFERENCE.md) |
| Mailbox notes and service registration | [docs/MAIL-REGISTRY.md](docs/MAIL-REGISTRY.md) |

## License and Sources

[LICENSE](LICENSE) · [NOTICE](NOTICE.md). See NOTICE for the applicable license scope.
