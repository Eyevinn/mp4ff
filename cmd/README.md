# mp4ff command line tools

The tools are built with the mp4ff library. Run a tool with `-h` to see all its options.

| Tool | Description |
| ---- | ----------- |
| [mp4ff-info](mp4ff-info) | Prints the box tree of an mp4 file with information about each box. With `-tree` or `-json`, and for files that cannot be decoded completely, the tree has box positions and errors. With `-brands`, it instead checks the ftyp and styp brands against the content. For an interactive view, see [mp4lens](https://github.com/Eyevinn/mp4lens). |
| [mp4ff-pslister](mp4ff-pslister) | Extracts and displays the parameter sets of AVC and HEVC (SPS, PPS, VPS) from an mp4 file, an Annex B byte stream or hex input, and the AV1 sequence header from an mp4 file. VVC is not supported. |
| [mp4ff-nallister](mp4ff-nallister) | Lists NAL units and picture types of AVC, HEVC or VVC video in a progressive or fragmented mp4 file, or in an Annex B byte stream. It can also print SEI messages. Video codecs without NAL units, such as AV1 and VP9, are not supported. |
| [mp4ff-subslister](mp4ff-subslister) | Lists details of wvtt, wvtc, stpp or stpc (WebVTT or TTML in ISOBMFF) subtitle samples. |
| [mp4ff-crop](mp4ff-crop) | Crops a **progressive** mp4 file to just before a sync sample after a given duration. |
| [mp4ff-encrypt](mp4ff-encrypt) | Encrypts a fragmented file with the cenc or cbcs Common Encryption scheme. |
| [mp4ff-decrypt](mp4ff-decrypt) | Decrypts a fragmented file encrypted with the cenc or cbcs Common Encryption scheme. |
| [mp4ff-mvhevc](mp4ff-mvhevc) | Inspects MV-HEVC (Multi-View HEVC) files and muxes HEVC (Annex B or mp4) into an MV-HEVC mp4. |
| [mp4ff-defragment](mp4ff-defragment) | Converts a fragmented mp4 file into a progressive mp4 file. |

## Installation

All tools are built for macOS, Linux and Windows (amd64 and arm64) on every
[release](https://github.com/Eyevinn/mp4ff/releases).

### Homebrew (macOS and Linux)

    brew install mp4ff

### Go

Install a tool directly from the repo (repeat for each tool you want), or run
`go install .` from the tool's directory:

    go install github.com/Eyevinn/mp4ff/cmd/mp4ff-info@latest
    go install github.com/Eyevinn/mp4ff/cmd/mp4ff-encrypt@latest

All tools are installed by running `go install ./cmd/...` in a clone of the repo.

The tools report the version that Go embeds when it builds them from a git checkout or
with `go install`. A build from a source archive, without git, reports `(devel)` unless
the version is set at link time:

    go build -ldflags "-X github.com/Eyevinn/mp4ff/internal.commitVersion=vX.Y.Z" ./cmd/mp4ff-info

### Linux packages (Debian/Ubuntu, Fedora/RHEL, Alpine)

Each release attaches `.deb`, `.rpm` and `.apk` packages (all tools in a single
`mp4ff` package) as well as plain `.tar.gz` archives. Download the one matching
your system from the [releases page](https://github.com/Eyevinn/mp4ff/releases)
and install it:

    sudo apt install ./mp4ff_<version>_linux_amd64.deb              # Debian/Ubuntu
    sudo rpm -i mp4ff_<version>_linux_amd64.rpm                     # Fedora/RHEL
    sudo apk add --allow-untrusted mp4ff_<version>_linux_amd64.apk  # Alpine

For any other distribution, download the `.tar.gz` archive and copy the
binaries into a directory on your `PATH`.

## Open Source Cloud

You can also run the tools as a job in
[Eyevinn Open Source Cloud](https://app.osaas.io/browse/eyevinn-mp4ff?utm_source=github&utm_medium=readme&utm_campaign=mp4ff).
Here is an example using the `mp4ff-crop` command and the Open Source Cloud CLI.

```bash
% export OSC_ACCESS_TOKEN=<your-personal-access-token>
% npx -y @osaas/cli@latest create eyevinn-mp4ff test \
  -o awsAccessKeyId=<s3-access-key-id> \
  -o awsSecretAccessKey=<s3-secret-key> \
  -o s3EndpointUrl=https://eyevinnlab-birme.minio-minio.auto.prod.osaas.io \
  -o cmdLineArgs="mp4ff-crop s3://input/VINN.mp4 s3://output/VINN-crop2.mp4"
```

The file VINN.mp4 on the bucket called "input" on the MinIO server at
https://eyevinnlab-birme.minio-minio.auto.prod.osaas.io is processed and the output is
uploaded to the bucket "output" on the same MinIO server.
