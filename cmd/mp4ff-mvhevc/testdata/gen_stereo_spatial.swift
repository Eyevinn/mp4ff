// Generator for stereo_spatial.mp4: a tiny genuine MV-HEVC (Apple spatial
// video) file encoded with AVFoundation/VideoToolbox on macOS (Apple Silicon).
// The frames are synthetic gradients, so the file is free of third-party
// content and can be redistributed with mp4ff.
//
// Regenerate with:
//   swiftc -O gen_stereo_spatial.swift -o gen_stereo_spatial
//   ./gen_stereo_spatial stereo_spatial.mp4 160 120 10 30
//
// The output contains an hvc1 sample entry with hvcC + lhvC (two layers,
// nuh_layer_id 0 and 1), vexu (stri/hero/cams/blin + unmodelled cmfy) and
// hfov, ctts (frame reordering), and no oinf/linf sample groups.
import AVFoundation
import CoreMedia
import CoreVideo
import VideoToolbox

let args = CommandLine.arguments
guard args.count >= 2 else {
    print("usage: gen_stereo_spatial <output.mp4> [width height frames fps]")
    exit(1)
}
let outURL = URL(fileURLWithPath: args[1])
let width = args.count > 2 ? Int(args[2])! : 160
let height = args.count > 3 ? Int(args[3])! : 120
let frames = args.count > 4 ? Int(args[4])! : 10
let fps = args.count > 5 ? Int(args[5])! : 30

try? FileManager.default.removeItem(at: outURL)

let writer = try AVAssetWriter(outputURL: outURL, fileType: .mp4)

let compressionProps: [CFString: Any] = [
    kVTCompressionPropertyKey_MVHEVCVideoLayerIDs: [0, 1],
    kVTCompressionPropertyKey_MVHEVCViewIDs: [0, 1],
    kVTCompressionPropertyKey_MVHEVCLeftAndRightViewIDs: [0, 1],
    kVTCompressionPropertyKey_HasLeftStereoEyeView: true,
    kVTCompressionPropertyKey_HasRightStereoEyeView: true,
    kVTCompressionPropertyKey_HeroEye: kCMFormatDescriptionHeroEye_Left,
    kVTCompressionPropertyKey_StereoCameraBaseline: 19240,       // micrometers
    kVTCompressionPropertyKey_HorizontalFieldOfView: 63400,      // thousandths of a degree
    kVTCompressionPropertyKey_HorizontalDisparityAdjustment: 200,
    kVTCompressionPropertyKey_AverageBitRate: 100_000,
]

let outputSettings: [String: Any] = [
    AVVideoCodecKey: AVVideoCodecType.hevc,
    AVVideoWidthKey: width,
    AVVideoHeightKey: height,
    AVVideoCompressionPropertiesKey: compressionProps,
]

let input = AVAssetWriterInput(mediaType: .video, outputSettings: outputSettings)
input.expectsMediaDataInRealTime = false

let sourceAttrs: [String: Any] = [
    kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA,
    kCVPixelBufferWidthKey as String: width,
    kCVPixelBufferHeightKey as String: height,
]
let adaptor = AVAssetWriterInputTaggedPixelBufferGroupAdaptor(
    assetWriterInput: input, sourcePixelBufferAttributes: sourceAttrs)

writer.add(input)
guard writer.startWriting() else {
    print("startWriting failed: \(String(describing: writer.error))")
    exit(1)
}
writer.startSession(atSourceTime: .zero)

func makeBuffer(pool: CVPixelBufferPool, frame: Int, eye: Int) -> CVPixelBuffer {
    var pb: CVPixelBuffer?
    CVPixelBufferPoolCreatePixelBuffer(nil, pool, &pb)
    let buf = pb!
    CVPixelBufferLockBaseAddress(buf, [])
    let base = CVPixelBufferGetBaseAddress(buf)!
    let stride = CVPixelBufferGetBytesPerRow(buf)
    for y in 0..<height {
        let row = base.advanced(by: y * stride).assumingMemoryBound(to: UInt8.self)
        for x in 0..<width {
            // moving gradient, offset horizontally per eye for "disparity"
            let v = UInt8((x * 2 + y + frame * 8 + eye * 12) & 0xff)
            row[x * 4 + 0] = v
            row[x * 4 + 1] = UInt8((y + frame * 4) & 0xff)
            row[x * 4 + 2] = eye == 0 ? 200 : 80
            row[x * 4 + 3] = 255
        }
    }
    CVPixelBufferUnlockBaseAddress(buf, [])
    return buf
}

for i in 0..<frames {
    while !input.isReadyForMoreMediaData { usleep(2000) }
    guard let pool = adaptor.pixelBufferPool else {
        print("no pixel buffer pool")
        exit(1)
    }
    let left = makeBuffer(pool: pool, frame: i, eye: 0)
    let right = makeBuffer(pool: pool, frame: i, eye: 1)
    let taggedBuffers: [CMTaggedBuffer] = [
        .init(tags: [.videoLayerID(0), .stereoView(.leftEye)], pixelBuffer: left),
        .init(tags: [.videoLayerID(1), .stereoView(.rightEye)], pixelBuffer: right),
    ]
    let pts = CMTime(value: CMTimeValue(i), timescale: CMTimeScale(fps))
    if !adaptor.appendTaggedBuffers(taggedBuffers, withPresentationTime: pts) {
        print("append failed at frame \(i): \(String(describing: writer.error))")
        exit(1)
    }
}

input.markAsFinished()
let sem = DispatchSemaphore(value: 0)
writer.finishWriting { sem.signal() }
sem.wait()
if writer.status != .completed {
    print("finishWriting failed: \(String(describing: writer.error))")
    exit(1)
}
print("wrote \(outURL.path)")
