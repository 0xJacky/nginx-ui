## Features

- None.

## Bug Fixes

- Stop site health checks from flooding the logs with expected `record not found` messages for configurations without database records, including backup files, while preserving remote namespace deployment status.
- Keep the database path synchronized when a stream is renamed, so deleting the renamed stream also removes its database record instead of leaving orphaned metadata.

## Contributors

@0xJacky
