// sampo-ocr — macOS Vision-framework OCR CLI used by sampo's OCR engine on darwin.
//
// Usage: sampo-ocr <image-path>
// Emits a JSON array of text blocks to stdout:
//   [{"text": "...", "confidence": 0.97, "x": 0.1, "y": 0.2, "w": 0.3, "h": 0.05}, ...]
// Bounding boxes are normalized (0..1), origin bottom-left (Vision convention).
//
// Build: swiftc -O scripts/sampo-ocr.swift -o bin/sampo-ocr
// Requires macOS 10.15+ (Vision text recognition).

import AppKit
import Foundation
import Vision

func fail(_ message: String, code: Int32) -> Never {
    FileHandle.standardError.write((message + "\n").data(using: .utf8)!)
    exit(code)
}

guard CommandLine.arguments.count > 1 else {
    fail("usage: sampo-ocr <image-path>", code: 2)
}

let imagePath = CommandLine.arguments[1]

guard let image = NSImage(contentsOfFile: imagePath),
    let cgImage = image.cgImage(forProposedRect: nil, context: nil, hints: nil)
else {
    fail("cannot load image: \(imagePath)", code: 1)
}

struct Block: Codable {
    let text: String
    let confidence: Float
    let x: Float
    let y: Float
    let w: Float
    let h: Float
}

let request = VNRecognizeTextRequest()
request.recognitionLevel = .accurate
request.usesLanguageCorrection = true

let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
do {
    try handler.perform([request])
} catch {
    fail("ocr failed: \(error)", code: 1)
}

var blocks: [Block] = []
for observation in request.results ?? [] {
    guard let candidate = observation.topCandidates(1).first else { continue }
    let box = observation.boundingBox
    blocks.append(
        Block(
            text: candidate.string,
            confidence: candidate.confidence,
            x: Float(box.minX),
            y: Float(box.minY),
            w: Float(box.width),
            h: Float(box.height)
        ))
}

let data = try JSONEncoder().encode(blocks)
FileHandle.standardOutput.write(data)
