# LinuxHello AI Models

This directory contains ONNX models required for face detection and recognition.

## Required Models

Download these models before using LinuxHello:

```bash
# Quick download (from project root)
make models
```

### 1. SCRFD Face Detection

**File**: `det_10g.onnx` (~16.9 MB)

```bash
curl -L -o buffalo_l.zip "https://huggingface.co/public-data/insightface/resolve/main/models/buffalo_l.zip"
unzip -j buffalo_l.zip det_10g.onnx
rm buffalo_l.zip
```

### 2. ArcFace Recognition

**File**: `arcface_r50.onnx` (~170 MB)

```bash
wget -O arcface_r50.onnx \
  "https://huggingface.co/lithiumice/insightface/resolve/main/models/buffalo_l/w600k_r50.onnx"
```

## Model Sources

- SCRFD: [InsightFace](https://github.com/deepinsight/insightface)
- ArcFace: [InsightFace Buffalo_L](https://github.com/deepinsight/insightface/tree/master/model_zoo)

## Note

Models are excluded from git due to size. Download them separately using the commands above or `make models`.
