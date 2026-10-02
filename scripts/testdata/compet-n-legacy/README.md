These two small nested archives are regression fixtures from the checksum-verified COMPET-N 2019-01-21 snapshot used by `fetch_compet_n.py`.

- `zip-implode.zip`: `compet-n/doom2/respawn/200-re08.zip`, using ZIP Implode compression.
- `arj-disguised-as-zip.zip`: `compet-n/doom2/nmare/nm23-058.zip`, containing an ARJ archive despite its filename.

The test checks the extracted demo SHA-256, map routing, and respawn flag. Legacy decoding uses optional `7z` or `7zz`; ordinary ZIP imports do not require either program. Extraction goes to stdout, with literal member names and size/CRC verification.

Snapshot SHA-256: `cb79b0a1bdc9233c2351d89fa229e00a5e461ae7d791ad535aea715673470fb1`.
