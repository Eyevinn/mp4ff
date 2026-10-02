# Benchmarks

The benchmarks live next to the code they measure, in `_test.go` files. Their results depend on the machine and the Go
version, so compare results from the same machine only.

## Running and comparing

Run a benchmark several times, with allocations, on two versions of the code:

```sh
go test ./mp4 -run '^$' -bench BenchmarkDecodeFragments -benchmem -count 8 > new.txt
```

Then compare the runs with [benchstat](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat):

```sh
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

The allocations are the heap allocations inside the benchmark loop. Most benchmarks read their input from
`mp4/testdata` before the loop.

## What is measured

| Benchmark | File | Measures |
|---|---|---|
| `BenchmarkDecodeFile`, `BenchmarkEncodeFile` | `mp4/benchmarks_test.go` | Decoding and encoding `1.m4s` and `prog_8s.mp4` through `io.Reader` and `io.Writer` |
| `BenchmarkDecodeFileSR`, `BenchmarkEncodeFileSW` | `mp4/benchmarks_srw_test.go` | The same through `bits.SliceReader` and `bits.SliceWriter` |
| `BenchmarkDecodeFragments` | `mp4/benchmarks_decode_test.go` | Decoding `1.m4s` as 60 one-sample fragments, the layout of low-latency streams, with `DecodeFileSR`, `DecodeFile` and the stream decoder |
| `BenchmarkEncodeFragments` | `mp4/benchmarks_encode_test.go` | Encoding the same 60 fragments with `Fragment.Encode` and `Fragment.EncodeSW` |
| `BenchmarkEncryptFragment` | `mp4/benchmarks_crypto_test.go` | Encrypting one AVC fragment in place with cenc and cbcs |
| `BenchmarkBuildFragment`, `BenchmarkBuildAndEncodeFragment` | `mp4/benchmarks_fragment_test.go` | Building a fragment from samples in memory with `AddFullSample` and `AddFullSamples`, and also encoding it |
| `BenchmarkFragmentFullSamples` | `mp4/append_samples_test.go` | Getting the samples of a decoded fragment with `GetFullSamples`, `AppendFullSamples` and `Samples` |
| `BenchmarkStreamSamples` | `mp4/append_samples_test.go` | Getting samples in the stream decoder with `GetSamples` and `AppendSamples` |
| `BenchmarkDecodeStsz`, `Ctts`, `Stco`, `Stss` | `mp4/benchmarks_table_test.go` | Decoding large sample tables |
| `BenchmarkDefragmentOverlapChain` | `mp4/defragmenter_test.go` | That defragmenting overlapping input stays linear |
| `BenchmarkParseNALUnits` | `avc/slice_test.go` | Parsing an AVC SPS and slice header, and `SliceHeaderSize` |
| `BenchmarkByteStreamToNaluSample` | `avc/annexb_test.go` | Converting an Annex B byte stream to length-prefixed NAL units |
| `BenchmarkWrite`, `BenchmarkEbspWrite` | `bits/writer_benchmark_test.go` | Writing bits, with and without emulation prevention |
| `BenchmarkDecodeCenc` | `cmd/mp4ff-decrypt/main_test.go` | Decrypting `prog_8s_enc_dashinit.mp4` |

## Results

### Current

Master at f60dbde, Go 1.27.1, Apple M4 Pro, n=6:

| Benchmark | time | bytes | allocations |
|---|---|---|---|
| `DecodeFileSR/1.m4s` | 922 ns | 2.3 KiB | 21 |
| `DecodeFileSR/prog_8s.mp4` | 5.3 µs | 12.6 KiB | 124 |
| `DecodeFile/1.m4s` | 6.7 µs | 60.2 KiB | 49 |
| `DecodeFile/prog_8s.mp4` | 45.2 µs | 460 KiB | 166 |
| `EncodeFileSW/1.m4s` | 1.2 µs | 64 B | 1 |
| `EncodeFileSW/prog_8s.mp4` | 11.4 µs | 64 B | 1 |
| `EncodeFile/1.m4s` | 880 ns | 88 B | 2 |
| `EncodeFile/prog_8s.mp4` | 11.6 µs | 8.6 KiB | 87 |

### Fewer allocations per fragment (unreleased)

These changes reduce the fixed cost of every fragment and sample, which dominates for low-latency streams with fragments
of one or a few samples. Apple M4 Pro, each compared with the code just before it:

| Change | Benchmark | time | allocations |
|---|---|---|---|
| Decoding ([#605](https://github.com/Eyevinn/mp4ff/pull/605)) | `DecodeFragments/DecodeFileSR` | 29.7 µs → 22.6 µs | 24.3 → 14.3 per fragment |
| | `DecodeFragments/DecodeFile` | 46.4 µs → 40.2 µs | 32.0 → 22.0 per fragment |
| | `DecodeFragments/StreamFile` | 31.7 µs → 27.4 µs | 28.3 → 16.2 per fragment |
| Encoding ([#613](https://github.com/Eyevinn/mp4ff/pull/613)) | `EncodeFragments/Encode` | 11.7 µs → 6.2 µs | 13 → 0 per fragment |
| | `EncodeFragments/EncodeSW` | 8.2 µs → 6.8 µs | 3 → 1 per fragment |
| EBSP reading ([#614](https://github.com/Eyevinn/mp4ff/pull/614)) | `ParseNALUnits/ParseSliceHeader` | 341 ns → 263 ns | 13 → 1 |
| | `ParseNALUnits/ParseSPSNALUnit` | 568 ns → 359 ns | 28 → 3 |
| | `ParseNALUnits/SliceHeaderSize` (new) | 244 ns | 0 |

The fragment benchmarks report the time for all 60 fragments.

### SliceReader and SliceWriter (v0.27)

Version 0.27 added the `Decode<X>SR` and `EncodeSW` methods. In these tables, v0.26 and v0.27 use `io.Reader` and
`io.Writer`, and v0.27-srw uses `bits.SliceReader` and `bits.SliceWriter`, whose benchmarks have been given the same
names as the others for the comparison. Encoding through `io.Writer` became slower in v0.27, since all writes then went
through the `SliceWriter` layer to avoid duplicated code. Using the `SliceWriter` directly allocates far less. The
machine and Go version of these runs were not recorded.

| name \ time/op | v0.26 | v0.27 | v0.27-srw |
|---|---|---|---|
| DecodeFile/1.m4s-16 | 21.9µs | 6.7µs | 2.6µs |
| DecodeFile/prog_8s.mp4-16 | 143µs | 48µs | 16µs |
| EncodeFile/1.m4s-16 | 1.70µs | 2.14µs | 1.50µs |
| EncodeFile/prog_8s.mp4-16 | 15.7µs | 18.4µs | 12.9µs |

| name \ alloc/op | v0.26 | v0.27 | v0.27-srw |
|---|---|---|---|
| DecodeFile/1.m4s-16 | 120kB | 28kB | 2kB |
| DecodeFile/prog_8s.mp4-16 | 906kB | 207kB | 12kB |
| EncodeFile/1.m4s-16 | 1.16kB | 1.39kB | 0.08kB |
| EncodeFile/prog_8s.mp4-16 | 6.84kB | 8.30kB | 0.05kB |

| name \ allocs/op | v0.26 | v0.27 | v0.27-srw |
|---|---|---|---|
| DecodeFile/1.m4s-16 | 98.0 | 42.0 | 34.0 |
| DecodeFile/prog_8s.mp4-16 | 454 | 180 | 169 |
| EncodeFile/1.m4s-16 | 15.0 | 15.0 | 3.0 |
| EncodeFile/prog_8s.mp4-16 | 101 | 86 | 1 |
