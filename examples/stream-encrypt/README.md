# stream-encrypt

HTTP streaming server that encrypts and refragments MP4 files on-the-fly using the `mp4.StreamFile` API.

## Features

- **HTTP Streaming**: Serves MP4 files via HTTP with chunked transfer encoding
- **Refragmentation**: Splits input fragments into smaller output fragments with configurable sample count
- **Encryption**: Encrypts fragments using Common Encryption (CENC or CBCS)
- **Low Latency**: Uses `GetSampleRange()` for minimal buffering and immediate delivery
- **Sequence Number Preservation**: Sub-fragments maintain the same sequence number as their parent

## Usage

### Basic Streaming (No Encryption, No Refragmentation)

```bash
go run *.go
curl http://localhost:8080/enc.mp4 -o output.mp4
```

### Using a Custom Input File

```bash
go run *.go -input /path/to/your/video.mp4
curl http://localhost:8080/enc.mp4 -o output.mp4
```

### Refragmentation

Split fragments to 30 samples each:

```bash
go run *.go -samples 30
curl http://localhost:8080/enc.mp4 -o refragmented.mp4
```

### Encryption with Refragmentation

```bash
go run *.go \
  -samples 30 \
  -key 11223344556677889900aabbccddeeff \
  -keyid 00112233445566778899aabbccddeeff \
  -iv 00000000000000000000000000000000 \
  -scheme cenc

curl http://localhost:8080/enc.mp4 -o encrypted.mp4
```

### Command-Line Options

```
  -input string
        Input MP4 file path (default "../../mp4/testdata/v300_multiple_segments.mp4")
  -port int
        HTTP server port (default 8080)
  -samples int
        Samples per fragment (0=no refrag) (default 0)
  -key string
        Encryption key (hex)
  -keyid string
        Key ID (hex)
  -iv string
        IV (hex)
  -scheme string
        Encryption scheme (cenc/cbcs) (default "cenc")
```

## How It Works

### Streaming Pipeline

```
Input File → StreamFile.InitDecodeStream() →
[Refragment?] → [Encrypt?] → HTTP Response (Chunked) → Client
```

1. **Init Segment**: Read and optionally modify for encryption, write immediately
2. **Fragment Processing**: For each input fragment:
   - Use `GetSampleRange()` to fetch only needed samples
   - Create sub-fragments if refragmentation enabled
   - Encrypt if encryption configured
   - Write and flush immediately

### Refragmentation Strategy

- **Input**: Fragment with 60 samples
- **Output** (samplesPerFrag=30): Two fragments with 30 samples each
- **Sequence Numbers**: Both sub-fragments keep the parent's sequence number
- **Benefits**: Lower latency, smaller chunk sizes for adaptive streaming

### Encryption

- **Schemes**: CENC (AES-CTR) or CBCS (AES-CBC)
- **IV Derivation**: Incremental IV per fragment based on fragment number
- **Metadata**: Adds `senc`, `saiz`, `saio` boxes to fragments
- **Init Modification**: Converts sample entries to `encv`/`enca`, adds `sinf` structure

## Testing

Run all tests:

```bash
go test -v
```

Individual test steps:

```bash
go test -v -run TestStep1  # Basic streaming
go test -v -run TestStep2  # Refragmentation
go test -v -run TestStep3  # Encryption
```

## Implementation Files

- **main.go**: HTTP server and request handling
- **refragment.go**: Fragment splitting using `GetSampleRange()`
- **encryptor.go**: Encryption setup and per-fragment encryption
- **main_test.go**: Integration tests for all features

## Design Principles

1. **Streaming-First**: Never buffer entire fragments, use `GetSampleRange()` instead of `GetSamples()`
2. **Memory Efficient**: Process and deliver each sub-fragment immediately
3. **Sequence Number Preservation**: All sub-fragments from same input share sequence number
4. **Reuse Existing Code**: Leverages `mp4.InitProtect()` and `mp4.EncryptFragment()`
