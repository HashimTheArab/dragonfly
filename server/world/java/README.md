# Java block conversion

`Parse`, `Resolve`, and `Legacy` convert Java block states into canonical states
from the caller's Bedrock registry. Unknown names, properties, metadata, and
states without a Bedrock equivalent return errors. Missing properties use the
Java block's declared defaults; supplied properties are never discarded.

The embedded table is generated from [PrismarineJS/minecraft-data](https://github.com/PrismarineJS/minecraft-data/tree/abfaadb4c45e2968a98fc927d32f7c648962d8ad),
revision `abfaadb4c45e2968a98fc927d32f7c648962d8ad`:

- `data/bedrock/1.26.30/blocksJ2B.json`: Java-to-Bedrock states.
- `data/pc/1.21.11/blocks.json`: Java default states and property values.
- `data/pc/common/legacy.json`: pre-flattening numeric block IDs and metadata.

Regenerate with `go generate ./server/world/java`. The generator pins its source
revision and writes deterministic gzip bytes. Default-state indices use the
property ordering documented by PrismarineJS/prismarine-block: the last property
varies fastest; Boolean zero means true.

The source data is published under the MIT license by the PrismarineJS
contributors (see the source repository's README). The compressed table retains
that attribution. It covers the source versions above, not arbitrary modded
blocks or future Java additions.
