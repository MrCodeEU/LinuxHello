# LinuxHello AI Models

This directory contains ONNX models required for face detection and recognition.

## Required Models

Download these models before using LinuxHello:

```bash
# Quick download (from project root)
make models
```

### 1. SCRFD Face Detection

**File**: `scrfd_person_2.5g.onnx` (~2.5 MB)

```bash
wget -O scrfd_person_2.5g.onnx \
  "https://github.com/deepinsight/insightface/releases/download/v0.7/scrfd_person_2.5g.onnx"
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
